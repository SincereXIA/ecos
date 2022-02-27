package gaia

import (
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"io"
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
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		logger.Infof("receive size: %v", len(r.Content))
		// TODO: 处理接收到的 Block，转发给同组 Node
	}
	return stream.SendAndClose(&common.Result{
		Status: common.Result_OK,
	})
}

func NewGaia(rpcServer *messenger.RpcServer) *Gaia {
	g := Gaia{}
	RegisterGaiaServer(rpcServer, &g)
	return &g
}
