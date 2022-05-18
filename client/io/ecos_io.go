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
	"ecos/utils/errno"
	"ecos/utils/logger"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/twmb/murmur3"
	"hash"
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
	pipes       *pipeline.ClusterPipelines
	bucketInfo  *infos.BucketInfo
	chunkPool   *common.Pool

	// for multipart upload
	multipartJobs sync.Map
}

// NewEcosIOFactory Constructor for EcosIOFactory
//
// nodeAddr shall provide the address to get ClusterInfo from
func NewEcosIOFactory(config *config.ClientConfig, volumeID, bucketName string) (*EcosIOFactory, error) {
	conn, err := messenger.GetRpcConn(config.NodeAddr, config.NodePort)
	if err != nil {
		return nil, err
	}
	watcherClient := watcher.NewWatcherClient(conn)
	reply, err := watcherClient.GetClusterInfo(context.Background(),
		&watcher.GetClusterInfoRequest{Term: 0})
	if err != nil {
		logger.Errorf("get group info fail: %v", err)
		return nil, err
	}
	clusterInfo := reply.GetClusterInfo()
	// TODO: Retry?
	pipes, err := pipeline.NewClusterPipelines(clusterInfo)
	if err != nil {
		logger.Errorf("get cluster pipelines fail: %v", err)
		return nil, err
	}
	ret := &EcosIOFactory{
		volumeID:   volumeID,
		bucketName: bucketName,

		infoAgent:   agent.NewInfoAgent(context.Background(), clusterInfo),
		clusterInfo: clusterInfo,
		config:      config,
		pipes:       pipes,
	}
	maxChunk := uint(ret.config.UploadBuffer / ret.config.Object.ChunkSize)
	chunkPool, _ := common.NewPool(ret.newLocalChunk, maxChunk, int(maxChunk))
	ret.chunkPool = chunkPool

	info, err := ret.infoAgent.Get(infos.InfoType_BUCKET_INFO, infos.GetBucketID(volumeID, bucketName))
	if err != nil {
		logger.Errorf("get bucket info fail: %v", err)
		return nil, err
	}
	ret.bucketInfo = info.BaseInfo().GetBucketInfo()
	return ret, err
}

func (f *EcosIOFactory) IsConnected() bool {
	return f.bucketInfo != nil
}

func (f *EcosIOFactory) newLocalChunk() (io.Closer, error) {
	return &localChunk{
		data:     make([]byte, f.config.Object.ChunkSize),
		freeSize: f.config.Object.ChunkSize,
	}, nil
}

// GetEcosWriter provide a EcosWriter for object associated with key
func (f *EcosIOFactory) GetEcosWriter(key string) *EcosWriter {
	var objHash hash.Hash
	switch f.bucketInfo.Config.ObjectHashType {
	case infos.BucketConfig_SHA256:
		objHash = sha256.New()
	case infos.BucketConfig_MURMUR3_128:
		objHash = murmur3.New128()
	case infos.BucketConfig_MURMUR3_32:
		objHash = murmur3.New32()
	default:
		objHash = nil
	}
	return &EcosWriter{
		infoAgent:      f.infoAgent,
		clusterInfo:    f.clusterInfo,
		bucketInfo:     f.bucketInfo,
		key:            key,
		config:         f.config,
		Status:         READING,
		pipes:          f.pipes,
		chunks:         f.chunkPool,
		blocks:         map[int]*Block{},
		objHash:        objHash,
		finishedBlocks: make(chan *Block),
	}
}

// CreateMultipartUploadJob Create a multipart upload job
//
// This function will return a jobID, which can be used to upload parts
func (f *EcosIOFactory) CreateMultipartUploadJob(key string) string {
	ret := f.GetEcosWriter(key)
	ret.partObject = true
	uploadId := uuid.New().String()
	f.multipartJobs.Store(uploadId, ret)
	return uploadId
}

// GetMultipartUploadWriter Get the writer for a multipart upload with a jobID
func (f *EcosIOFactory) GetMultipartUploadWriter(jobID string) (*EcosWriter, error) {
	ret, ok := f.multipartJobs.Load(jobID)
	if !ok {
		return nil, errno.NoSuchUpload
	}
	return ret.(*EcosWriter), nil
}

// AbortMultipartUploadJob Abort a multipart upload job
func (f *EcosIOFactory) AbortMultipartUploadJob(jobID string) error {
	writer, err := f.GetMultipartUploadWriter(jobID)
	if err != nil {
		return err
	}
	err = writer.Abort()
	if err != nil {
		return err
	}
	f.multipartJobs.Delete(jobID)
	return nil
}

// CompleteMultipartUploadJob Complete a multipart upload job
func (f *EcosIOFactory) CompleteMultipartUploadJob(jobID string, parts ...types.CompletedPart) (string, error) {
	// Check if the parts is in order
	for i, part := range parts {
		var prevPart types.CompletedPart
		if i != 0 {
			prevPart = parts[i-1]
		}
		if prevPart.PartNumber > part.PartNumber {
			return "", errno.InvalidPartOrder
		}
	}
	writer, err := f.GetMultipartUploadWriter(jobID)
	if err != nil {
		return "", err
	}
	etag, err := writer.CloseMultiPart(parts...)
	if err != nil {
		return "", err
	}
	f.multipartJobs.Delete(jobID)
	return etag, nil
}

// AbortAllMultipartUploadJob Abort all multipart upload job
func (f *EcosIOFactory) AbortAllMultipartUploadJob() error {
	f.multipartJobs.Range(func(key, value interface{}) bool {
		writer, err := f.GetMultipartUploadWriter(key.(string))
		if err != nil {
			return true
		}
		err = writer.Abort()
		if err != nil {
			return true
		}
		f.multipartJobs.Delete(key.(string))
		return true
	})
	return nil
}

// ListMultipartUploadJob List all multipart upload job
func (f *EcosIOFactory) ListMultipartUploadJob() ([]types.MultipartUpload, error) {
	ret := make([]types.MultipartUpload, 0)
	f.multipartJobs.Range(func(key, value interface{}) bool {
		ret = append(ret, types.MultipartUpload{
			Key:      &value.(*EcosWriter).key,
			UploadId: common.PtrString(key.(string)),
		})
		return true
	})
	return ret, nil
}

// GetEcosReader provide a EcosWriter for object associated with key
func (f *EcosIOFactory) GetEcosReader(key string) *EcosReader {
	return &EcosReader{
		infoAgent:     f.infoAgent,
		clusterInfo:   f.clusterInfo,
		bucketInfo:    f.bucketInfo,
		key:           key,
		pipes:         f.pipes,
		curBlockIndex: 0,
		meta:          nil,
		config:        f.config,
	}
}
