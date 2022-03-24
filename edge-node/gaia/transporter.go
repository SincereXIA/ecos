package gaia

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	commonMessenger "ecos/messenger/common"
	"ecos/utils/common"
	"ecos/utils/errno"
	"io"
	"os"
	"path"
)

// PrimaryCopyTransporter 负责将 Block 写入本地以及 pipeline 中其余节点
type PrimaryCopyTransporter struct {
	info     *object.BlockInfo
	pipeline *pipeline.Pipeline

	localWriter   io.Writer
	remoteWriters []io.Writer
	ctx           context.Context
}

// NewPrimaryCopyTransporter return a PrimaryCopyTransporter
//
// when pipeline contains only self node, it will only write to local
func NewPrimaryCopyTransporter(ctx context.Context, info *object.BlockInfo, pipeline *pipeline.Pipeline,
	selfID uint64, groupInfo *infos.GroupInfo, basePath string) (t *PrimaryCopyTransporter, err error) {
	var localWriter io.Writer
	// 创建远端 writer
	var remoteWriters []io.Writer
	for _, nodeID := range pipeline.RaftId {
		if selfID == nodeID {
			// 创建本地 writer
			blockPath := path.Join(basePath, info.BlockId)
			_ = common.InitParentPath(blockPath)
			localWriter, err = os.Create(blockPath)
			if err != nil {
				return nil, err // TODO: trans to ecos err
			}
			continue
		}
		nodeInfo := getNodeInfo(groupInfo, nodeID)
		if nodeInfo == nil {
			// TODO: return err
		}
		var writer *RemoteWriter
		writer, err = NewRemoteWriter(ctx, info, nodeInfo, pipeline)
		if err != nil {
			// TODO: return err
		}
		remoteWriters = append(remoteWriters, writer)
	}

	return &PrimaryCopyTransporter{
		ctx:           ctx,
		info:          info,
		pipeline:      pipeline,
		localWriter:   localWriter,
		remoteWriters: remoteWriters,
	}, nil
}

func (transporter *PrimaryCopyTransporter) Write(chunk []byte) (n int, err error) {
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

// RemoteWriter change byte flow to Gaia rpc request and send it.
type RemoteWriter struct {
	stream    Gaia_UploadBlockDataClient
	blockInfo *object.BlockInfo
}

// NewRemoteWriter return a new RemoteWriter.
func NewRemoteWriter(ctx context.Context, blockInfo *object.BlockInfo, nodeInfo *infos.NodeInfo,
	p *pipeline.Pipeline) (*RemoteWriter, error) {
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		// TODO: return err
	}
	client := NewGaiaClient(conn)
	stream, err := client.UploadBlockData(ctx)
	err = stream.Send(&UploadBlockRequest{
		Payload: &UploadBlockRequest_Message{
			Message: &ControlMessage{
				Code:     ControlMessage_BEGIN,
				Block:    blockInfo,
				Pipeline: makeSingleNodePipeline(nodeInfo, p),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &RemoteWriter{stream: stream, blockInfo: blockInfo}, err
}

func (w *RemoteWriter) Write(chunk []byte) (n int, err error) {
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

func getNodeInfo(groupInfo *infos.GroupInfo, nodeID uint64) *infos.NodeInfo {
	for _, info := range groupInfo.NodesInfo {
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
