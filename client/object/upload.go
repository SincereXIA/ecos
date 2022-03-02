package object

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/gaia"
	"ecos/edge-node/object"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"os"
	"time"
)

var ClientConfig config.Config

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
	if configTimeout := ClientConfig.UploadTimeoutMs; configTimeout > 0 {
		newClient.context, newClient.cancel = context.WithTimeout(context.Background(),
			time.Duration(ClientConfig.UploadTimeoutMs)*time.Millisecond)
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
		logger.Infof("Result: %#v", result)
	}
	return result, err
}

func (c *UploadClient) sliceAndUpload(file *os.File, key string) ([]*object.BlockInfo, int, error) {
	var blocks []*object.BlockInfo
	filename := file.Name()
	blockCount := 1
	for {
		err := c.NewUploadStream()
		if err != nil {
			return nil, 0, err
		}
		buffer := make([]byte, ClientConfig.Object.BlockSize)
		n, err := file.Read(buffer)
		if n == 0 {
			if err == io.EOF {
				break
			} else {
				return nil, 0, err
			}
		}
		newBlock := NewBlock("", blockCount, n, buffer[:n])
		blocks = append(blocks, &newBlock.BlockInfo)
		logger.Debugf("Uploading %vth Block of %v", blockCount, filename)
		err = newBlock.Upload(c.stream)
		if err != nil {
			return blocks, blockCount, err
		}
		res, err := c.stream.CloseAndRecv()
		if err != nil {
			logger.Errorf("Close upload stream failed!: %v", err)
			return blocks, blockCount, err
		}
		logger.Debugf("Upload Result for block %v: %v", blockCount, res.Message)
		blockCount += 1
	}
	defer func(totalBlock int) {
		logger.Infof("Uploaded %v Blocks!", totalBlock)
	}(blockCount - 1)
	return blocks, blockCount - 1, nil
}

// PutObject provides the way to send an ordinary object through gRpc
//
// 0. Establish the connection with a Gaia Server and get the stream connection
//
// 1. Slicing object into 4M Blocks and then 1M Chunks
//
// 2. Sending these Chunks through grpc.ClientStream
//
// 3. CloseSend then get result from Server of this Block
//
// 4. Repeat Until all Block's have been sent
func PutObject(localFilePath string, server string, key string) ([]*object.BlockInfo, int, error) {
	// Step 0
	client, err := NewGaiaClient(server)
	if err != nil {
		return nil, 0, err
	}
	if client.cancel != nil {
		defer client.cancel()
	}
	// Step 1, 2
	file, err := os.Open(localFilePath)
	if err != nil {
		return nil, 0, err
	}
	blocks, totalBlocks, err := client.sliceAndUpload(file, key)
	if err != nil {
		logger.Warningf("Upload Interrupted with error: %v", err)
		return blocks, totalBlocks, err
	}
	return blocks, totalBlocks, nil
}
