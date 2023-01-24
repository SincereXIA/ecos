package gaia

import (
	"bufio"
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	commonMessenger "ecos/messenger/common"
	"ecos/utils/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"io"
	"os"
	"path"
)

// PrimaryCopyTransporter 负责将 Block 写入本地以及 pipeline 中其余节点
type PrimaryCopyTransporter struct {
	info     *object.BlockInfo
	pipeline *pipeline.Pipeline
	basePath string

	localWriter   io.Writer
	remoteWriters []io.Writer
	ctx           context.Context
}

// NewPrimaryCopyTransporter return a PrimaryCopyTransporter
//
// when pipeline contains only self node, it will only write to local
func NewPrimaryCopyTransporter(ctx context.Context, info *object.BlockInfo, pipeline *pipeline.Pipeline,
	selfID uint64, clusterInfo *infos.ClusterInfo, basePath string) (t *PrimaryCopyTransporter, err error) {
	transporter := &PrimaryCopyTransporter{
		info:     info,
		pipeline: pipeline,
		basePath: basePath,
		ctx:      ctx,
	}
	// 创建远端 writer
	var remoteWriters []io.Writer
	for _, nodeID := range pipeline.RaftId {
		if selfID == nodeID {
			continue
		}
		nodeInfo := getNodeInfo(clusterInfo, nodeID)
		if nodeInfo == nil {
			// TODO: return err
			logger.Errorf("NodeInfo not found: %v", nodeID)
			continue
		}
		var writer *RemoteWriter
		writer, err = NewRemoteWriter(ctx, info, nodeInfo, pipeline)
		if err != nil {
			logger.Errorf("NewRemoteWriter err: %v", err)
			return nil, err
		}
		remoteWriters = append(remoteWriters, writer)
	}
	logger.Debugf("Gaia: %v create remoteWriters: %v", selfID, remoteWriters)
	transporter.remoteWriters = remoteWriters
	return transporter, nil
}

func (transporter *PrimaryCopyTransporter) GetStoragePath() string {
	return path.Join(transporter.basePath, transporter.info.BlockId)
}

func (transporter *PrimaryCopyTransporter) init() error {
	// 创建本地 writer
	blockPath := transporter.GetStoragePath()
	_ = common.InitParentPath(blockPath)
	localWriter, err := os.Create(blockPath)
	bufferedWriter := bufio.NewWriter(localWriter)
	if err != nil {
		return err // TODO: trans to ecos err
	}
	transporter.localWriter = bufferedWriter
	return nil
}

func (transporter *PrimaryCopyTransporter) Write(chunk []byte) (n int, err error) {
	if transporter.localWriter == nil {
		err := transporter.init()
		if err != nil {
			return 0, err
		}
	}
	// MultiWriter 貌似不是并行的
	multiWriter := io.MultiWriter(append(transporter.remoteWriters, transporter.localWriter)...)
	return multiWriter.Write(chunk)
}

func (transporter *PrimaryCopyTransporter) Close() error {
	switch localWriter := transporter.localWriter.(type) {
	case *os.File:
		err := localWriter.Sync()
		if err != nil {
			return err
		}
		err = localWriter.Close()
		if err != nil {
			return err
		}
	}
	for _, remoteWriter := range transporter.remoteWriters {
		err := remoteWriter.(*RemoteWriter).Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (transporter *PrimaryCopyTransporter) Delete() (error, int64) {
	localPath := transporter.GetStoragePath()
	for _, remoteWriter := range transporter.remoteWriters {
		err := remoteWriter.(*RemoteWriter).Delete()
		if err != nil {
			return err, 0
		}
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err, 0
	}
	fi, _ := file.Stat()
	fileSize := fi.Size()
	err = os.Remove(localPath)
	return err, fileSize
}

// RemoteWriter change byte flow to Gaia rpc request and send it.
type RemoteWriter struct {
	ctx       context.Context
	client    GaiaClient
	stream    Gaia_UploadBlockDataClient
	blockInfo *object.BlockInfo
	pipeline  *pipeline.Pipeline
}

// NewRemoteWriter return a new RemoteWriter.
func NewRemoteWriter(ctx context.Context, blockInfo *object.BlockInfo, nodeInfo *infos.NodeInfo,
	p *pipeline.Pipeline) (*RemoteWriter, error) {
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		// TODO: return err
	}
	client := NewGaiaClient(conn)
	return &RemoteWriter{
		ctx:       ctx,
		client:    client,
		blockInfo: blockInfo,
		pipeline:  makeSingleNodePipeline(nodeInfo, p),
	}, err
}

func (w *RemoteWriter) init() error {
	stream, err := w.client.UploadBlockData(w.ctx)
	if err != nil {
		logger.Errorf("transporter remoteWriter init remote stream err: %v", err.Error())
		return err
	}
	err = stream.Send(&UploadBlockRequest{
		Payload: &UploadBlockRequest_Message{
			Message: &ControlMessage{
				Code:     ControlMessage_BEGIN,
				Block:    w.blockInfo,
				Pipeline: w.pipeline,
			},
		},
	})
	w.stream = stream
	return err
}

func (w *RemoteWriter) Write(chunk []byte) (n int, err error) {
	if w.stream == nil {
		err := w.init()
		if err != nil {
			return 0, err
		}
	}
	err = w.stream.Send(&UploadBlockRequest{
		Payload: &UploadBlockRequest_Chunk{
			Chunk: &Chunk{
				Content: chunk,
			},
		},
	})
	if err != nil {
		return 0, err
	}
	return len(chunk), nil
}

func (w *RemoteWriter) Close() error {
	if w.stream == nil {
		return errno.RemoteGaiaFail
	}
	err := w.stream.Send(&UploadBlockRequest{
		Payload: &UploadBlockRequest_Message{
			Message: &ControlMessage{
				Code:  ControlMessage_EOF,
				Block: w.blockInfo,
			},
		},
	})
	result, err := w.stream.CloseAndRecv()
	if err != nil || result.Status != commonMessenger.Result_OK {
		return errno.RemoteGaiaFail
	}
	return err
}

func (w *RemoteWriter) Delete() error {
	_, err := w.client.DeleteBlock(w.ctx, &DeleteBlockRequest{
		BlockId:  w.blockInfo.BlockId,
		Pipeline: w.pipeline,
	})
	if err != nil {
		return errno.RemoteGaiaFail
	}
	return nil
}

func getNodeInfo(clusterInfo *infos.ClusterInfo, nodeID uint64) *infos.NodeInfo {
	for _, info := range clusterInfo.NodesInfo {
		if info.RaftId == nodeID {
			return info
		}
	}
	return nil
}

func makeSingleNodePipeline(info *infos.NodeInfo, p *pipeline.Pipeline) *pipeline.Pipeline {
	return &pipeline.Pipeline{
		PgId:     p.PgId,
		RaftId:   []uint64{info.RaftId},
		SyncType: pipeline.Pipeline_STREAM,
	}
}
