package object

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/alaya"
	"ecos/edge-node/object"
	"ecos/messenger/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type MetaClient struct {
	serverAddr string
	client     alaya.AlayaClient
	context    context.Context
	cancel     context.CancelFunc
}

// NewMetaClient create a client with config
func NewMetaClient(serverAddr string) (*MetaClient, error) {
	var newClient = MetaClient{
		serverAddr: serverAddr,
	}
	conn, err := grpc.Dial(newClient.serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	newClient.client = alaya.NewAlayaClient(conn)
	if configTimeout := config.Config.UploadTimeoutMs; configTimeout > 0 {
		newClient.context, newClient.cancel = context.WithTimeout(context.Background(),
			time.Duration(config.Config.UploadTimeoutMs)*time.Millisecond)
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
