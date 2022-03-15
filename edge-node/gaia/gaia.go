package gaia

import (
	"context"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"io"
)

// Gaia save object block data
// 盖亚存储和查询对象 block 数据
// 地势坤，厚德载物
type Gaia struct {
	UnimplementedGaiaServer

	ctx         context.Context
	cancel      context.CancelFunc
	selfInfo    *node.NodeInfo
	infoStorage node.InfoStorage
}

func (g *Gaia) UploadBlockData(stream Gaia_UploadBlockDataServer) error {
	var transporter *PrimaryCopyTransporter
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
		case *UploadBlockRequest_Message:
			msg := payload.Message
			code := msg.Code
			p := msg.Pipeline
			switch code {
			case ControlMessage_BEGIN:
				// TODO: 建立与同组 Node 的连接，准备转发
				transporter, err = NewPrimaryCopyTransporter(g.ctx, msg.Block, p, g.selfInfo.RaftId,
					g.infoStorage.GetGroupInfo(msg.Term))
				if err != nil {
					return err
				}
			case ControlMessage_EOF:
				// TODO: 确认转发成功，关闭连接
				err := transporter.Close()
				if err != nil {
					return stream.SendAndClose(&common.Result{
						Status:  common.Result_FAIL,
						Message: err.Error(),
					})
				}
				return stream.SendAndClose(&common.Result{
					Status: common.Result_OK,
				})
			default:
				logger.Errorf("ControlMessage has unexpected code %v", code)
				return stream.SendAndClose(&common.Result{
					Status: common.Result_FAIL,
				})
			}
		case *UploadBlockRequest_Chunk:
			// TODO: 处理接收到的 Block，转发给同组 Node
			chunk := payload.Chunk.Content
			if transporter == nil {
				return stream.SendAndClose(&common.Result{
					Status:  common.Result_FAIL,
					Code:    errno.NoTransporterErr.Code,
					Message: errno.NoTransporterErr.Error(),
				})
			}
			_, err = transporter.Write(chunk)
			if err != nil {
				return stream.SendAndClose(&common.Result{
					Status:  common.Result_FAIL,
					Code:    errno.CodeTransporterWriteFail,
					Message: errno.TransporterWriteFail.Error() + err.Error(),
				})
			}
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
	}
}

func NewGaia(rpcServer *messenger.RpcServer, selfInfo *node.NodeInfo, infoStorage node.InfoStorage) *Gaia {
	ctx, cancel := context.WithCancel(context.Background())
	g := Gaia{
		ctx:         ctx,
		cancel:      cancel,
		selfInfo:    selfInfo,
		infoStorage: infoStorage,
	}
	RegisterGaiaServer(rpcServer, &g)
	return &g
}
