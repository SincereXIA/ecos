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

func (c *CloudMoon) ProposeInfo(ctx context.Context, req *moon.ProposeInfoRequest) (*moon.ProposeInfoReply, error) {
	info := req.BaseInfo

	resp, err := c.r.SendRequestDirect(&Request{
		Method:    Request_PUT,
		Resource:  Request_INFO,
		RequestId: req.Id,
		InfoType:  info.GetInfoType(),
		Info:      info,
	})
	if err != nil {
		logger.Errorf("CloudMoon send request to edge failed: %s", err.Error())
		return nil, err
	}

	_ = <-resp
	return &moon.ProposeInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
	}, nil
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
		BaseInfo: info,
	}, nil
}

func (c *CloudMoon) ListInfo(ctx context.Context, req *moon.ListInfoRequest) (*moon.ListInfoReply, error) {
	resp, err := c.r.SendRequestDirect(&Request{
		Method:    Request_LIST,
		Resource:  Request_INFO,
		RequestId: req.Prefix,
		InfoType:  req.InfoType,
	})
	if err != nil {
		logger.Errorf("CloudMoon send request to edge failed: %s", err.Error())
		return nil, err
	}

	r := <-resp
	if len(r.Infos) == 0 {
		return nil, errors.New(r.Result.Message)
	}

	var infos []*infos.BaseInfo
	for _, info := range r.Infos {
		infos = append(infos, info.BaseInfo())
	}

	return &moon.ListInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		BaseInfos: infos,
	}, nil
}

func (c *CloudMoon) GetInfoDirect(infoType infos.InfoType, id string) (*infos.BaseInfo, error) {
	resp, err := c.r.SendRequestDirect(&Request{
		Method:    Request_GET,
		Resource:  Request_INFO,
		RequestId: id,
		InfoType:  infoType,
	})
	if err != nil {
		logger.Errorf("CloudMoon send request to edge failed: %s", err.Error())
		return infos.InvalidInfo{}.BaseInfo(), err
	}

	r := <-resp
	if len(r.Infos) == 0 {
		return infos.InvalidInfo{}.BaseInfo(), errors.New(r.Result.Message)
	}

	info := r.Infos[0]
	return info, nil
}
