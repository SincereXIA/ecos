package object

import (
	"ecos/client/user"
	"ecos/edge-node/gaia"
	"ecos/utils/logger"
	"errors"
	"github.com/google/uuid"
	"io"
)

type BlockInf struct {
	blockId  string
	objectId string
	pgId     string
	size     uint64
}

type Block struct {
	BlockInf
	data []byte
}

func (b Block) toChunks() []*gaia.Chunk {
	chunkSize := ClientConfig.Object.ChunkSize
	var chunks []*gaia.Chunk
	offset := uint64(0)
	for offset < b.size {
		noMore := false
		nextOffset := offset + chunkSize
		if nextOffset > b.size {
			nextOffset = b.size
			noMore = true
		}
		chunks = append(chunks, &gaia.Chunk{
			Content:  b.data[offset:nextOffset],
			NoMore:   noMore,
			ObjectId: b.objectId,
			BlockId:  b.blockId,
			PgId:     b.pgId,
		})
		offset = nextOffset
	}
	return chunks
}

func (b Block) Upload(stream gaia.Gaia_UploadBlockDataClient) error {
	chunks := b.toChunks()
	byteCount := uint64(0)
	for _, chunk := range chunks {
		err := stream.Send(chunk)
		if err != nil && err != io.EOF {
			logger.Errorf("uploadBlock error: %v", err)
			return err
		}
		byteCount += uint64(len(chunk.Content))
	}
	if byteCount != b.size {
		logger.Errorf("Incompatible size: Chunks: %v, Block: %v", byteCount, b.size)
		return errors.New("incompatible upload size")
	}
	logger.Infof("Sent %v bytes in block", byteCount)
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
