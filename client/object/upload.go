package object

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/gaia"
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
	newClient.context, newClient.cancel = context.WithTimeout(context.Background(),
		time.Duration(ClientConfig.UploadTimeoutMs)*time.Millisecond)
	return &newClient, err
}

func (c *UploadClient) NewUploadStream() error {
	var err error
	c.stream, err = c.client.UploadBlockData(c.context)
	return err
}

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

func (c *UploadClient) sliceAndUpload(file *os.File, key string) ([]BlockInf, int, error) {
	var blocks []BlockInf
	filename := file.Name()
	blockCount := 1
	for {
		buffer := make([]byte, ClientConfig.Object.BlockSize)
		n, err := file.Read(buffer)
		if n == 0 {
			if err == io.EOF {
				break
			} else {
				return nil, 0, err
			}
		}
		newBlock := Block{BlockInf: BlockInf{
			blockId:  GenBlockId("", blockCount),
			objectId: GenObjectId(key),
			pgId:     "",
			size:     uint64(n),
		},
			data: buffer[:n],
		}
		blocks = append(blocks, newBlock.BlockInf)
		logger.Infof("Uploading %vth Block of %v", blockCount, filename)
		err = newBlock.Upload(c.stream)
		if err != nil {
			return blocks, blockCount, err
		}
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
// 3. CloseSend then get result from Server
func PutObject(localFilePath string, server string, key string) ([]BlockInf, int, error) {
	// Step 0
	client, err := NewGaiaClient(server)
	if err != nil {
		return nil, 0, err
	}
	defer client.cancel()
	err = client.NewUploadStream()
	if err != nil {
		return nil, 0, err
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
	// Step 3
	result, err := client.GetUploadResult()
	if err != nil {
		logger.Warningf("Upload Finished with Result: %v", err)
		return blocks, totalBlocks, err
	}
	logger.Infof("Upload Success! %v -> %v", localFilePath, key)
	logger.Infof("Upload Result: %v", result.Message)
	return blocks, totalBlocks, nil
}
