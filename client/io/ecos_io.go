package io

import (
	"context"
	"crypto/sha256"
	"ecos/client/config"
	agent "ecos/client/info-agent"
	"ecos/edge-node/infos"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/logger"
	"io"
	"sync"
)

// EcosIOFactory Generates EcosWriter with ClientConfig
type EcosIOFactory struct {
	volumeID   string
	bucketName string

	infoAgent   *agent.InfoAgent
	config      *config.ClientConfig
	clusterInfo *infos.ClusterInfo
	objPipes    []*pipeline.Pipeline
	blockPipes  []*pipeline.Pipeline
	bucketInfo  *infos.BucketInfo
	chunkPool   *common.Pool
	blockPool   *sync.Pool
}

// NewEcosIOFactory Constructor for EcosIOFactory
//
// nodeAddr shall provide the address to get ClusterInfo from
func NewEcosIOFactory(config *config.ClientConfig, volumeID, bucketName string) *EcosIOFactory {
	conn, err := messenger.GetRpcConn(config.NodeAddr, config.NodePort)
	if err != nil {
		return nil
	}
	watcherClient := watcher.NewWatcherClient(conn)
	reply, err := watcherClient.GetClusterInfo(context.Background(),
		&watcher.GetClusterInfoRequest{Term: 0})
	if err != nil {
		logger.Errorf("get group info fail: %v", err)
		return nil
	}
	clusterInfo := reply.GetClusterInfo()
	// TODO: Retry?
	// TODO: Get pgNum, groupNum from moon
	ret := &EcosIOFactory{
		volumeID:   volumeID,
		bucketName: bucketName,

		infoAgent:   agent.NewInfoAgent(context.Background(), clusterInfo),
		clusterInfo: clusterInfo,
		config:      config,
		objPipes:    pipeline.GenPipelines(*clusterInfo, objPgNum, groupNum),
		blockPipes:  pipeline.GenPipelines(*clusterInfo, blockPgNum, groupNum),
		blockPool:   &sync.Pool{},
	}
	maxChunk := uint(ret.config.UploadBuffer / ret.config.Object.ChunkSize)
	chunkPool, _ := common.NewPool(ret.newLocalChunk, maxChunk, int(maxChunk))
	ret.chunkPool = chunkPool

	info, err := ret.infoAgent.Get(infos.InfoType_BUCKET_INFO, infos.GetBucketID(volumeID, bucketName))
	if err != nil {
		logger.Errorf("get bucket info fail: %v", err)
		return nil
	}
	ret.bucketInfo = info.BaseInfo().GetBucketInfo()
	ret.blockPool.New = func() interface{} {
		return make([]byte, 0, ret.bucketInfo.Config.BlockSize)
	}
	return ret
}

func (f *EcosIOFactory) newLocalChunk() (io.Closer, error) {
	return &localChunk{
		data:     make([]byte, f.config.Object.ChunkSize),
		freeSize: f.config.Object.ChunkSize,
	}, nil
}

// GetEcosWriter provide a EcosWriter for object associated with key
func (f *EcosIOFactory) GetEcosWriter(key string) EcosWriter {
	return EcosWriter{
		infoAgent:      f.infoAgent,
		clusterInfo:    f.clusterInfo,
		bucketInfo:     f.bucketInfo,
		key:            key,
		config:         f.config,
		Status:         READING,
		chunks:         f.chunkPool,
		blocks:         map[int]*Block{},
		blockPipes:     f.blockPipes,
		objHash:        sha256.New(),
		objPipes:       f.objPipes,
		finishedBlocks: make(chan *Block),
	}
}

// GetEcosReader provide a EcosWriter for object associated with key
func (f *EcosIOFactory) GetEcosReader(key string) *EcosReader {
	return &EcosReader{
		infoAgent:     f.infoAgent,
		clusterInfo:   f.clusterInfo,
		bucketInfo:    f.bucketInfo,
		key:           key,
		blockPipes:    nil,
		curBlockIndex: 0,
		meta:          nil,
		objPipes:      f.objPipes,
		config:        f.config,
		blockPool:     f.blockPool,
	}
}
