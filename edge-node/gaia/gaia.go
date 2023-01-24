package gaia

import (
	"bufio"
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/gaia"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"github.com/rcrowley/go-metrics"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"
)

// Gaia save object block data
// 盖亚存储和查询对象 block 数据
// 地势坤，厚德载物
type Gaia struct {
	gaia.UnimplementedGaiaServer

	ctx     context.Context
	cancel  context.CancelFunc
	watcher *watcher.Watcher

	config *Config

	transMu sync.Mutex
}

func (g *Gaia) UploadBlockData(stream gaia.Gaia_UploadBlockDataServer) error {
	transporter := &gaia.PrimaryCopyTransporter{}
	var timeStart *time.Time
	defer func() {
		if timeStart != nil {
			metrics.GetOrRegisterTimer(watcher.MetricsGaiaBlockPutTimer, nil).UpdateSince(*timeStart)
		}
	}()
	for {
		select {
		case <-g.ctx.Done():
			return stream.SendAndClose(&common.Result{
				Status:  common.Result_FAIL,
				Code:    errno.CodeGaiaClosed,
				Message: errno.GaiaClosedErr.Error(),
			})
		default:
		}

		r, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch payload := r.Payload.(type) {
		case *gaia.UploadBlockRequest_Message:
			err = g.processControlMessage(payload, transporter, stream)
			if payload.Message.Code == gaia.ControlMessage_BEGIN && len(payload.Message.Pipeline.RaftId) == 3 {
				t := time.Now()
				timeStart = &t
			}
		case *gaia.UploadBlockRequest_Chunk:
			err = g.processChunk(payload, transporter, stream)
		case nil:
			// The field is not set.
			logger.Warningf("Received blank Payload")
			return stream.SendAndClose(&common.Result{
				Status: common.Result_FAIL,
			})
		default:
			logger.Errorf("UploadBlockRequest.Payload has unexpected type %T", payload)
			return stream.SendAndClose(&common.Result{
				Status: common.Result_FAIL,
			})
		}
		if err != nil {
			return err
		}
	}
}

func (g *Gaia) getBlockBasePath() string {
	return path.Join(g.config.BasePath)
}

// GetBlockData return local block data to client
func (g *Gaia) GetBlockData(req *gaia.GetBlockRequest, server gaia.Gaia_GetBlockDataServer) error {
	timeStart := time.Now()
	// open block file
	blockPath := path.Join(g.config.BasePath, req.BlockId)
	block, err := os.Open(blockPath)
	if err != nil {
		logger.Errorf("open blockPath: %v failed, err: %v", blockPath, err)
		return err
	}
	defer block.Close()

	// send block data to client
	r := bufio.NewReader(block)
	startChunk := req.CurChunk
	curChunk := 0
	chunkSize := g.config.ChunkSize
	chunk := make([]byte, chunkSize)
	for {
		readBytes, err := r.Read(chunk)
		if err != nil && err != io.EOF {
			logger.Errorf("read panic, err: $v", err)
			return err
		}
		// read this block finished, return io.EOF
		if err == io.EOF {
			logger.Infof("gaia [%v] read this block finished", g.watcher.GetSelfInfo().RaftId)
			metrics.GetOrRegisterTimer(watcher.MetricsGaiaBlockGetTimer, nil).UpdateSince(timeStart)
			break
		}
		if uint64(curChunk) < startChunk {
			curChunk++
			continue
		}
		err = server.Send(&gaia.GetBlockResult{
			Payload: &gaia.GetBlockResult_Chunk{
				Chunk: &gaia.Chunk{
					Content:   chunk[:readBytes],
					ReadBytes: uint64(readBytes),
				},
			},
		})
		if err != nil {
			logger.Infof("send Block res err: %v", err)
			return err
		}
	}
	return nil
}

func (g *Gaia) DeleteBlock(_ context.Context, req *gaia.DeleteBlockRequest) (*common.Result, error) {
	clusterInfo, err := g.watcher.GetClusterInfoByTerm(req.Term)
	if err != nil {
		logger.Errorf("get cluster info by term: %v failed, err: %v", req.Term, err)
		return nil, err
	}
	blockInfo := &object.BlockInfo{
		BlockId: req.BlockId,
	}
	t, err := gaia.NewPrimaryCopyTransporter(g.ctx, blockInfo, req.Pipeline, g.watcher.GetSelfInfo().RaftId,
		&clusterInfo, g.config.BasePath)
	if err != nil {
		logger.Errorf("new primary copy transporter failed, err: %v", err)
		return nil, err
	}
	err, blockSize := t.Delete()
	if err != nil {
		logger.Errorf("delete block failed, err: %v", err)
		return nil, err
	}
	logger.Infof("delete block: %v success", req.BlockId)
	metrics.GetOrRegisterCounter(watcher.MetricsGaiaBlockCount, nil).Dec(1)
	metrics.GetOrRegisterCounter("exp_gaia_size", nil).Dec(blockSize)
	return &common.Result{Status: common.Result_OK}, nil
}

func (g *Gaia) transformBlocks(info infos.Information) {
	// open block file in basePath
	logger.Infof("gaia start transform block")
	clusterInfo := info.BaseInfo().GetClusterInfo()
	chunkBuffer := make([]byte, g.config.ChunkSize)

	g.transMu.Lock()
	defer g.transMu.Unlock()
	defer logger.Infof("gaia transform block finished")
	basePath := g.config.BasePath
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		logger.Warningf("read dir: %v failed, err: %v", basePath, err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := path.Join(basePath, file.Name())
		f, err := os.Open(filePath)
		if err != nil {
			logger.Warningf("open file: %v failed, err: %v", filePath, err)
			continue
		}
		defer f.Close()
		logger.Infof("transform block: %v", file.Name())
		blockId := file.Name()
		pgID := object.GenBlockPgID(blockId, clusterInfo.BlockPgNum)
		blockInfo := &object.BlockInfo{
			BlockId: blockId,
			PgId:    pgID,
		}
		clusterPipelines, err := pipeline.NewClusterPipelines(*clusterInfo)
		if err != nil {
			logger.Errorf("new cluster pipelines failed, err: %v", err)
			return
		}
		pipeline := clusterPipelines.GetBlockPipeline(pgID)

		// get block transporter
		t, err := gaia.NewPrimaryCopyTransporter(g.ctx, blockInfo, pipeline, g.watcher.GetSelfInfo().RaftId,
			clusterInfo, g.config.BasePath)
		if err != nil {
			logger.Errorf("new primary copy transporter failed, err: %v", err)
			t.Close()
			continue
		}
		// transform block

		_, _ = io.CopyBuffer(t, f, chunkBuffer)
		t.Close()
	}
}

// processControlMessage will modify transporter when receive ControlMessage_BEGIN
func (g *Gaia) processControlMessage(message *gaia.UploadBlockRequest_Message, transporter *gaia.PrimaryCopyTransporter,
	stream gaia.Gaia_UploadBlockDataServer) (err error) {
	msg := message.Message
	code := msg.Code
	p := msg.Pipeline
	switch code {
	case gaia.ControlMessage_BEGIN:
		// 建立与同组 Node 的连接，准备转发
		clusterInfo, err := g.watcher.GetClusterInfoByTerm(msg.Term)
		if err != nil {
			return err
		}
		t, err := gaia.NewPrimaryCopyTransporter(g.ctx, msg.Block, p, g.watcher.GetSelfInfo().RaftId,
			&clusterInfo, g.config.BasePath)
		if err != nil {
			return err
		}
		*transporter = *t
		logger.Infof("Gaia %v start receive block: %v", g.watcher.GetSelfInfo().RaftId, msg.Block.BlockId)
	case gaia.ControlMessage_EOF:
		// 确认转发成功，关闭连接
		err := transporter.Close()
		if err != nil {
			return stream.SendAndClose(&common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			})
		}
		logger.Infof("Gaia %v save block: %v success", g.watcher.GetSelfInfo().RaftId, msg.Block.BlockId)
		metrics.GetOrRegisterCounter(watcher.MetricsGaiaBlockCount, nil).Inc(1)
		metrics.GetOrRegisterCounter("exp_gaia_size", nil).Inc(int64(msg.Block.BlockSize))
		return stream.SendAndClose(&common.Result{
			Status: common.Result_OK,
		})
	default:
		logger.Errorf("ControlMessage has unexpected code %v", code)
		return stream.SendAndClose(&common.Result{
			Status: common.Result_FAIL,
		})
	}
	return nil
}

func (g *Gaia) processChunk(chunk *gaia.UploadBlockRequest_Chunk, transporter *gaia.PrimaryCopyTransporter,
	stream gaia.Gaia_UploadBlockDataServer) (err error) {
	data := chunk.Chunk.Content
	if transporter == nil {
		return stream.SendAndClose(&common.Result{
			Status:  common.Result_FAIL,
			Code:    errno.NoTransporterErr.Code,
			Message: errno.NoTransporterErr.Error(),
		})
	}
	_, err = transporter.Write(data)
	if err != nil {
		return stream.SendAndClose(&common.Result{
			Status:  common.Result_FAIL,
			Code:    errno.CodeTransporterWriteFail,
			Message: errno.TransporterWriteFail.Error() + err.Error(),
		})
	}
	return nil
}

func NewGaia(ctx context.Context, rpcServer *messenger.RpcServer, watcher *watcher.Watcher,
	config *Config) *Gaia {
	ctx, cancel := context.WithCancel(ctx)
	g := Gaia{
		ctx:     ctx,
		cancel:  cancel,
		watcher: watcher,
		config:  config,
	}
	gaia.RegisterGaiaServer(rpcServer, &g)
	//err := watcher.SetOnInfoUpdate(infos.InfoType_CLUSTER_INFO, "gaia"+strconv.Itoa(rand.Int()), g.transformBlocks)
	//if err != nil {
	//	return nil
	//}
	return &g
}
