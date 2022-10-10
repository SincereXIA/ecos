package rainbow

import (
	"bufio"
	"context"
	"ecos/cloud/config"
	"ecos/common/gaia"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"io"
	"os"
	"path"
)

type CloudGaia struct {
	ctx context.Context
	gaia.UnimplementedGaiaServer
	conf *config.CloudConfig
}

func NewCloudGaia(ctx context.Context, rpcServer *messenger.RpcServer, conf *config.CloudConfig) *CloudGaia {
	g := &CloudGaia{
		ctx:  ctx,
		conf: conf,
	}
	gaia.RegisterGaiaServer(rpcServer, g)
	return g
}

func (g *CloudGaia) UploadBlockData(stream gaia.Gaia_UploadBlockDataServer) error {
	transporter := &gaia.PrimaryCopyTransporter{}
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

func (g *CloudGaia) DeleteBlock(ctx context.Context, in *gaia.DeleteBlockRequest) (*common.Result, error) {
	panic("implement me")
}

func (g *CloudGaia) GetBlockData(req *gaia.GetBlockRequest, server gaia.Gaia_GetBlockDataServer) error {
	logger.Infof("Cloud Gaia start send block: %v", req.BlockId)
	// open block file
	blockPath := path.Join(g.conf.BasePath, "gaia", "blocks", req.BlockId)
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
	chunkSize := g.conf.ChunkSize
	chunk := make([]byte, chunkSize)
	for {
		readBytes, err := r.Read(chunk)
		if err != nil && err != io.EOF {
			logger.Errorf("read panic, err: $v", err)
			return err
		}
		// read this block finished, return io.EOF
		if err == io.EOF {
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

// processControlMessage will modify transporter when receive ControlMessage_BEGIN
func (g *CloudGaia) processControlMessage(message *gaia.UploadBlockRequest_Message, transporter *gaia.PrimaryCopyTransporter,
	stream gaia.Gaia_UploadBlockDataServer) (err error) {
	msg := message.Message
	code := msg.Code
	p := &pipeline.Pipeline{
		RaftId: []uint64{0},
	}
	switch code {
	case gaia.ControlMessage_BEGIN:
		// 建立 Transporter 保存到本地
		t, err := gaia.NewPrimaryCopyTransporter(g.ctx, msg.Block, p, 0,
			nil, path.Join(g.conf.BasePath, "gaia", "blocks"))
		if err != nil {
			return err
		}
		*transporter = *t
		logger.Infof("Cloud Gaia start receive block: %v", msg.Block.BlockId)
	case gaia.ControlMessage_EOF:
		// 确认转发成功，关闭连接
		err := transporter.Close()
		if err != nil {
			return stream.SendAndClose(&common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			})
		}
		logger.Infof("Cloud Gaia save block: %v success", msg.Block.BlockId)
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

func (g *CloudGaia) processChunk(chunk *gaia.UploadBlockRequest_Chunk, transporter *gaia.PrimaryCopyTransporter,
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
