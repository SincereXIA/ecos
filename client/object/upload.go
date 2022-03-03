package object

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/gaia"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type UploadClient struct {
	serverAddr string
	client     gaia.GaiaClient
	context    context.Context
	cancel     context.CancelFunc
	stream     gaia.Gaia_UploadBlockDataClient
}

// NewGaiaClient creates a client stream with 1s Timeout
func NewGaiaClient(serverAddr string) (*UploadClient, error) {
	var newClient UploadClient
	newClient.serverAddr = serverAddr
	conn, err := grpc.Dial(newClient.serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	newClient.client = gaia.NewGaiaClient(conn)
	if configTimeout := config.Config.UploadTimeoutMs; configTimeout > 0 {
		newClient.context, newClient.cancel = context.WithTimeout(context.Background(),
			time.Duration(config.Config.UploadTimeoutMs)*time.Millisecond)
	} else {
		newClient.context = context.Background()
		newClient.cancel = nil
	}
	return &newClient, err
}

// NewUploadStream establish the stream connection for Chunk Upload
func (c *UploadClient) NewUploadStream() error {
	var err error
	c.stream, err = c.client.UploadBlockData(c.context)
	return err
}

// GetUploadResult return the result of last closed object uploading
func (c *UploadClient) GetUploadResult() (*common.Result, error) {
	err := c.stream.CloseSend()
	if err != nil {
		logger.Errorf("Close grpc stream err: %v", err)
		return nil, err
	} else {
		logger.Infof("Closed grpc stream")
	}
	result, err := c.stream.CloseAndRecv()
	if err != nil {
		logger.Errorf("Unable to get result form server: %v", err)
		return nil, err
	} else {
		logger.Infof("Result: %v", result)
	}
	return result, err
}
