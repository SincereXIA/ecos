package io

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/messenger"
	"ecos/messenger/common"
)

type MetaClient struct {
	serverNode *infos.NodeInfo
	client     alaya.AlayaClient
	context    context.Context
	cancel     context.CancelFunc
}

// NewMetaClient create a client with config
func NewMetaClient(ctx context.Context, serverNode *infos.NodeInfo, conf *config.ClientConfig) (*MetaClient, error) {
	var newClient = MetaClient{
		serverNode: serverNode,
	}
	conn, err := messenger.GetRpcConnByNodeInfo(serverNode)
	if err != nil {
		return nil, err
	}
	newClient.client = alaya.NewAlayaClient(conn)
	if configTimeout := conf.UploadTimeout; configTimeout > 0 {
		newClient.context, newClient.cancel = context.WithTimeout(ctx, configTimeout)
	} else {
		newClient.context = ctx
		newClient.cancel = nil
	}
	return &newClient, err
}

// SubmitMeta provides a way to upload ObjectMeta to AlayaServer
func (c *MetaClient) SubmitMeta(meta *object.ObjectMeta) (*common.Result, error) {
	return c.client.RecordObjectMeta(c.context, meta)
}

// GetObjMeta provides a way to get ObjectMeta from AlayaServer
func (c *MetaClient) GetObjMeta(objId string) (*object.ObjectMeta, error) {
	metaReq := alaya.MetaRequest{
		ObjId: objId,
	}
	return c.client.GetObjectMeta(c.context, &metaReq)
}
