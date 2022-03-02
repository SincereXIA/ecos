package gaia

import (
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
)

// Gaia save object block data
// 盖亚存储和查询对象 block 数据
// 地势坤，厚德载物
type Gaia struct {
	UnimplementedGaiaServer
}

func (g *Gaia) UploadBlockData(stream Gaia_UploadBlockDataServer) error {
	for {
		r, err := stream.Recv()
		if err != nil {
			return err
		}
		switch payload := r.Payload.(type) {
		case *UploadBlockRequest_Message:
			msg := payload.Message
			code := msg.Code
			switch code {
			case ControlMessage_BEGIN:
				// TODO: 建立与同组 Node 的连接，准备转发
				return stream.SendAndClose(&common.Result{
					Status: common.Result_OK,
				})
			case ControlMessage_EOF:
				// TODO: 确认转发成功，关闭连接
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
			return stream.SendAndClose(&common.Result{
				Status: common.Result_OK,
			})
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

func NewGaia(rpcServer *messenger.RpcServer) *Gaia {
	g := Gaia{}
	RegisterGaiaServer(rpcServer, &g)
	return &g
}
