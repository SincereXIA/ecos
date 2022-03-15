package object

import (
	"crypto/sha256"
	"ecos/client/user"
	"ecos/edge-node/gaia"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
)

type BlockStatus int

const (
	READING BlockStatus = iota
	UPLOADING
	FAILED
	FINISHED
)

type Block struct {
	object.BlockInfo

	// status describes current status of block
	status BlockStatus

	chunks []*localChunk

	// These infos are for BlockInfo
	key        string
	groupInfo  *node.GroupInfo
	blockCount int
	needHash   bool

	// These are for Upload and Release
	uploadCount int
	delFunc     func(*Block)
}

// Upload provides a way to upload self to a given stream
func (b *Block) Upload(stream gaia.Gaia_UploadBlockDataClient) error {
	// Start Upload by ControlMessage with Code BEGIN
	start := &gaia.UploadBlockRequest{
		Payload: &gaia.UploadBlockRequest_Message{
			Message: &gaia.ControlMessage{
				Code:  gaia.ControlMessage_BEGIN,
				Block: &b.BlockInfo,
			},
		},
	}
	err := stream.Send(start)
	if err != nil && err != io.EOF {
		logger.Errorf("uploadBlock error: %v", err)
		return err
	}
	byteCount := uint64(0)
	for _, chunk := range b.chunks {
		err = stream.Send(&gaia.UploadBlockRequest{
			Payload: &gaia.UploadBlockRequest_Chunk{
				Chunk: &gaia.Chunk{Content: chunk.data},
			},
		})
		if err != nil && err != io.EOF {
			logger.Errorf("uploadBlock error: %v", err)
			return err
		}
		byteCount += uint64(len(chunk.data))
	}
	if byteCount != b.BlockSize {
		logger.Errorf("Incompatible size: Chunks: %v, Block: %v", byteCount, b.Size)
		return errors.New("incompatible upload size")
	}
	logger.Infof("Sent %v bytes in block", byteCount)
	// End Upload by ControlMessage with Code EOF
	end := &gaia.UploadBlockRequest{
		Payload: &gaia.UploadBlockRequest_Message{
			Message: &gaia.ControlMessage{
				Code:  gaia.ControlMessage_EOF,
				Block: &b.BlockInfo,
			},
		},
	}
	err = stream.Send(end)
	if err != nil && err != io.EOF {
		logger.Errorf("uploadBlock error: %v", err)
		return err
	}
	return nil
}

// genBlockHash Block.genBlockHash uses SHA256 to calc the hash value of block content
func (b *Block) genBlockHash() error {
	if b.status != UPLOADING {
		return errno.IllegalStatus
	}
	if !b.needHash {
		b.BlockHash = ""
		return nil
	}
	sha256h := sha256.New()
	for _, chunk := range b.chunks {
		sha256h.Write(chunk.data)
	}
	b.BlockHash = hex.EncodeToString(sha256h.Sum(nil))
	return nil
}

func minSize(i int, i2 int) int {
	if i < i2 {
		return i
	}
	return i2
}

func (b *Block) updateBlockInfo() error {
	b.BlockInfo.BlockId = GenBlockId(b.key, b.blockCount)
	b.BlockInfo.BlockSize = 0
	for _, chunk := range b.chunks {
		b.BlockInfo.BlockSize += uint64(len(chunk.data))
	}
	err := b.genBlockHash()
	if err != nil {
		return err
	}
	// TODO: Calculate Place Group from Block Info and GroupInfo
	b.BlockInfo.PgId = GenBlockPG(&b.BlockInfo)
	return nil
}

func (b *Block) getUploadStream() (*UploadClient, error) {
	serverInfo := clientNode.LocalInfoStorage.GetNodeInfo(0, b.blockPipes[b.BlockInfo.PgId].RaftId[0])
	client, err := NewGaiaClient(serverInfo)
	if err != nil {
		logger.Errorf("Unable to start Gaia Client: %v", err)
		return nil, err
	}
	err = client.NewUploadStream()
	if err != nil {
		logger.Errorf("Unable to start upload stream: %v", err)
		return nil, err
	}
	return client, nil
}

func (b *Block) Close() error {
	if b.status != READING {
		return errno.RepeatedClose
	}
	b.status = UPLOADING
	err := b.updateBlockInfo()
	if err != nil {
		return err
	}
	client, err := b.getUploadStream()
	if err != nil {
		return err
	}
	go func() {
		// TODO: Should We make this go routine repeating when a upload is failed?
		if client.cancel != nil {
			defer client.cancel()
		}
		err = b.Upload(client.stream)
		if err != nil {
			b.status = FAILED
			return
		} else {
			_, err := client.GetUploadResult()
			if err != nil {
				b.status = FAILED
				return
			}
			b.status = FINISHED
			defer b.delFunc(b)
		}
	}()
	return nil
}

// GenObjectId Generates ObjectId for a given object
func GenObjectId(key string) string {
	return user.GetUserVolume() + "/" + user.GetUserBucket() + key
}

// GenBlockId Generates BlockId for the `i` th block of a specific object
// This method ensures the global unique
func GenBlockId(objectId string, blockCount int) string {
	return uuid.New().String()
}

// GenObjectPG Generates PgId for ObjectMeta
// PgId of ObjectMeta depends on `key` of Object
func GenObjectPG(key string) uint64 {
	// TODO: Calculate ObjectMeta.PgId based on Object key
	return 1
}

// GenBlockPG Generates PgId for Block
// PgId of Block depends on `BlockId` of Block
// While BlockId depends on `Block.BlockHash`
func GenBlockPG(block *object.BlockInfo) uint64 {
	// TODO: Calculate BlockInfo.PgId based on Object key
	return 2
}
