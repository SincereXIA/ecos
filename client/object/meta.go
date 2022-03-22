package object

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/alaya"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/messenger"
	"ecos/messenger/common"
)

type MetaClient struct {
	serverNode *node.NodeInfo
	client     alaya.AlayaClient
	context    context.Context
	cancel     context.CancelFunc
}

// NewMetaClient create a client with config
func NewMetaClient(serverNode *node.NodeInfo) (*MetaClient, error) {
	var newClient = MetaClient{
		serverNode: serverNode,
	}
	conn, err := messenger.GetRpcConnByInfo(serverNode)
	if err != nil {
		return nil, err
	}
	newClient.client = alaya.NewAlayaClient(conn)
	if configTimeout := config.Config.UploadTimeout; configTimeout > 0 {
		newClient.context, newClient.cancel = context.WithTimeout(context.Background(), configTimeout)
	} else {
		newClient.context = context.Background()
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
