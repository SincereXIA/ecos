package io

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/gaia"
	"ecos/utils/logger"
)

type UploadClient struct {
	client  gaia.GaiaClient
	context context.Context
	cancel  context.CancelFunc
	stream  gaia.Gaia_UploadBlockDataClient
}

// NewGaiaClient creates a client stream with 1s Timeout
func NewGaiaClient(serverInfo *infos.NodeInfo, conf *config.ClientConfig) (*UploadClient, error) {
	var newClient UploadClient
	conn, err := messenger.GetRpcConnByNodeInfo(serverInfo)
	if err != nil {
		return nil, err
	}
	newClient.client = gaia.NewGaiaClient(conn)
	if configTimeout := conf.UploadTimeout; configTimeout > 0 {
		newClient.context, newClient.cancel = context.WithTimeout(context.Background(), configTimeout)
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
	// TODO (xiong): cannot get recv because context deadline exceeded
	result, err := c.stream.CloseAndRecv()
	if err != nil {
		logger.Errorf("Unable to get result form server: %v", err)
		return nil, err
	} else {
		logger.Infof("Block upload success")
	}
	return result, err
}
