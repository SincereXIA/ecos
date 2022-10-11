package rainbow

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/moon"
	"ecos/utils/logger"
	"errors"
)

type CloudMoon struct {
	moon.UnimplementedMoonServer
	ctx context.Context
	r   *Rainbow
}

func NewCloudMoon(ctx context.Context, rpcServer *messenger.RpcServer, r *Rainbow) *CloudMoon {
	m := &CloudMoon{
		ctx: ctx,
		r:   r,
	}
	moon.RegisterMoonServer(rpcServer, m)
	return m
}

func (c *CloudMoon) GetInfo(ctx context.Context, req *moon.GetInfoRequest) (*moon.GetInfoReply, error) {
	info, err := c.GetInfoDirect(req.InfoType, req.InfoId)
	if err != nil {
		return nil, err
	}
	return &moon.GetInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		BaseInfo: info.BaseInfo(),
	}, nil
}

func (c *CloudMoon) GetInfoDirect(infoType infos.InfoType, id string) (infos.Information, error) {
	resp, err := c.r.SendRequestDirect(&Request{
		Method:    Request_GET,
		Resource:  Request_INFO,
		RequestId: id,
		InfoType:  infoType,
	})
	if err != nil {
		logger.Errorf("CloudMoon send request to edge failed: %s", err.Error())
		return infos.InvalidInfo{}, err
	}

	r := <-resp
	if len(r.Infos) == 0 {
		return infos.InvalidInfo{}, errors.New(r.Result.Message)
	}

	info := r.Infos[0]
	return info.BaseInfo(), nil
}
