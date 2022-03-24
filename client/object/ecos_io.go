package object

import (
	"context"
	"crypto/sha256"
	"ecos/client/config"
	clientNode "ecos/client/node"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/logger"
	"io"
)

// EcosIOFactory Generates EcosWriter with ClientConfig
type EcosIOFactory struct {
	config     *config.ClientConfig
	groupInfo  *node.GroupInfo
	objPipes   []*pipeline.Pipeline
	blockPipes []*pipeline.Pipeline
}

// NewEcosIOFactory Constructor for EcosIOFactory
//
// nodeAddr shall provide the address to get GroupInfo from
func NewEcosIOFactory(config *config.ClientConfig) *EcosIOFactory {
	conn, err := messenger.GetRpcConn(config.NodeAddr, config.NodePort)
	if err != nil {
		return nil
	}
	moonClient := moon.NewMoonClient(conn)
	groupInfo, err := moonClient.GetGroupInfo(context.Background(), &moon.GetGroupInfoRequest{Term: 0})
	if err != nil {
		logger.Errorf("get group info fail: %v", err)
		return nil
	}
	// TODO: Retry?
	clientNode.InfoStorage.SaveGroupInfoWithTerm(0, groupInfo)

	// TODO: Get pgNum, groupNum from moon
	ret := &EcosIOFactory{
		groupInfo:  groupInfo,
		config:     config,
		objPipes:   pipeline.GenPipelines(groupInfo, objPgNum, groupNum),
		blockPipes: pipeline.GenPipelines(groupInfo, blockPgNum, groupNum),
	}
	return ret
}

func (f *EcosIOFactory) newLocalChunk() (io.Closer, error) {
	return &localChunk{freeSize: f.config.Object.ChunkSize}, nil
}

// GetEcosWriter provide a EcosWriter for object associated with key
func (f *EcosIOFactory) GetEcosWriter(key string) *EcosWriter {
	maxChunk := uint(f.config.UploadBuffer / f.config.Object.ChunkSize)
	chunkPool, _ := common.NewPool(f.newLocalChunk, maxChunk, int(maxChunk))
	return &EcosWriter{
		groupInfo:  f.groupInfo,
		key:        key,
		config:     f.config,
		Status:     READING,
		chunks:     chunkPool,
		blocks:     map[int]*Block{},
		blockPipes: f.blockPipes,
		meta: &object.ObjectMeta{
			ObjId:      "",
			ObjSize:    0,
			UpdateTime: nil,
			ObjHash:    "",
			PgId:       0,
			Blocks:     []*object.BlockInfo{},
		},
		objHash:        sha256.New(),
		objPipes:       f.objPipes,
		finishedBlocks: make(chan *Block),
	}
}

// GetEcosReader provide a EcosWriter for object associated with key
func (f *EcosIOFactory) GetEcosReader(key string) *EcosReader {
	maxChunkId := f.config.Object.BlockSize/f.config.Object.ChunkSize - 1
	return &EcosReader{
		groupInfo:         f.groupInfo,
		key:               key,
		blockPipes:        nil,
		curBlockId:        0,
		meta:              nil,
		objPipes:          f.objPipes,
		curChunkIdInBlock: 0,
		maxChunkIdInBlock: maxChunkId,
		curChunk:          0,
		sizeOfChunk:       f.config.Object.ChunkSize,
		chunkOffset:       0,
		alreadyReadBytes:  0,
	}
}
