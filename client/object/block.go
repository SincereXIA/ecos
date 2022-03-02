package object

import (
	"crypto/sha256"
	"ecos/client/user"
	"ecos/edge-node/gaia"
	"ecos/edge-node/object"
	"ecos/utils/logger"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"io"
)

type Block struct {
	object.BlockInfo
	data []byte
}

// NewBlock Creates new Block with BlockInfo
//
// The BlockHash is CALCULATED from data
//
// object: ObjectId of current Block
//
// blockCount: ordinal number of the Block in Object
//
// size: size of Block data field
//
// data: content of Block
func NewBlock(objectId string, blockCount int, size int, data []byte) *Block {
	ret := &Block{BlockInfo: object.BlockInfo{
		BlockId: GenBlockId(objectId, blockCount),
		Size:    uint64(size),
	},
		data: data,
	}
	ret.PgId = GenBlockPG(&ret.BlockInfo)
	ret.genBlockHash()
	return ret
}

func (b *Block) toChunks() []*gaia.Chunk {
	chunkSize := ClientConfig.Object.ChunkSize
	var chunks []*gaia.Chunk
	offset := uint64(0)
	for offset < b.Size {
		nextOffset := offset + chunkSize
		if nextOffset > b.Size {
			nextOffset = b.Size
		}
		chunks = append(chunks, &gaia.Chunk{
			Content: b.data[offset:nextOffset],
		})
		offset = nextOffset
	}
	return chunks
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
	chunks := b.toChunks()
	byteCount := uint64(0)
	for _, chunk := range chunks {
		err = stream.Send(&gaia.UploadBlockRequest{
			Payload: &gaia.UploadBlockRequest_Chunk{Chunk: chunk},
		})
		if err != nil && err != io.EOF {
			logger.Errorf("uploadBlock error: %v", err)
			return err
		}
		byteCount += uint64(len(chunk.Content))
	}
	if byteCount != b.Size {
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
func (b *Block) genBlockHash() {
	sha256h := sha256.New()
	sha256h.Write(b.data)
	b.BlockHash = hex.EncodeToString(sha256h.Sum(nil))
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
