package client

import (
	"context"
	"ecos/client/config"
	info_agent "ecos/client/info-agent"
	"ecos/client/io"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/logger"
	"errors"
	lru "github.com/hashicorp/golang-lru"
	"path"
	"strconv"
	"sync"
)

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc

	config      *config.ClientConfig
	clusterInfo *infos.ClusterInfo
	infoAgent   *info_agent.InfoAgent

	factoryPool *lru.Cache

	mutex sync.RWMutex
}

const (
	// DefaultFactoryPoolSize is the default size of factory pool
	DefaultFactoryPoolSize = 10
)

func New(config *config.ClientConfig) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	conn, err := messenger.GetRpcConn(config.NodeAddr, config.NodePort)
	if err != nil {
		cancel()
		logger.Errorf("connect to node failed: %s", err.Error())
		return nil, errors.New("connect to edge node failed")
	}
	watcherClient := watcher.NewWatcherClient(conn)
	reply, err := watcherClient.GetClusterInfo(context.Background(),
		&watcher.GetClusterInfoRequest{Term: 0})
	if err != nil {
		cancel()
		logger.Errorf("get cluster info failed: %s", err.Error())
		return nil, errors.New("get cluster info failed")
	}
	clusterInfo := reply.GetClusterInfo()

	lruPool, err := lru.NewWithEvict(DefaultFactoryPoolSize, func(key interface{}, value interface{}) {
		switch inter := value.(type) {
		case *io.EcosIOFactory:
			err := inter.AbortAllMultipartUploadJob()
			if err != nil {
				return
			}
		}
	})
	if err != nil {
		cancel()
		logger.Errorf("create factory pool failed: %s", err.Error())
		return nil, errors.New("create factory pool failed")
	}
	return &Client{
		ctx:         ctx,
		config:      config,
		cancel:      cancel,
		clusterInfo: clusterInfo,
		infoAgent:   info_agent.NewInfoAgent(ctx, clusterInfo),
		factoryPool: lruPool,
	}, nil
}

func (client *Client) GetClusterInfo() {
	client.mutex.RLock()
	defer client.mutex.RUnlock()
}

func (client *Client) ListObjects(_ context.Context, bucketName, prefix string) ([]*object.ObjectMeta, error) {
	userID := client.config.Credential.GetUserID()
	bucketID := infos.GenBucketID(userID, bucketName)
	info, err := client.infoAgent.Get(infos.InfoType_BUCKET_INFO, bucketID)
	if err != nil {
		return nil, err
	}
	bucketInfo := info.BaseInfo().GetBucketInfo()
	p := pipeline.GenMetaPipelines(*client.clusterInfo)
	var result []*object.ObjectMeta
	for i := 1; int32(i) <= bucketInfo.Config.KeySlotNum; i++ {
		pgID := object.GenSlotPgID(bucketInfo.GetID(), int32(i), client.clusterInfo.MetaPgNum)
		nodeID := p[pgID-1].RaftId[0]
		info, err := client.infoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(nodeID, 10))
		if err != nil {
			return nil, err
		}
		conn, err := messenger.GetRpcConnByNodeInfo(info.BaseInfo().GetNodeInfo())
		if err != nil {
			return nil, err
		}
		alayaClient := alaya.NewAlayaClient(conn)
		ctx, _ := alaya.SetTermToContext(client.ctx, client.clusterInfo.Term)
		reply, err := alayaClient.ListMeta(ctx, &alaya.ListMetaRequest{
			Prefix: path.Join(bucketInfo.GetID(), strconv.Itoa(i), prefix),
		})
		if err != nil {
			return nil, err
		}
		result = append(result, reply.Metas...)
	}
	return result, nil
}

func (client *Client) GetIOFactory(bucketName string) (*io.EcosIOFactory, error) {
	if ret, ok := client.factoryPool.Get(bucketName); ok && ret != nil {
		ecosIOFactory := ret.(*io.EcosIOFactory)
		if ecosIOFactory.IsConnected() {
			return ecosIOFactory, nil
		} else {
			client.factoryPool.Remove(bucketName)
		}
	}
	ret, err := io.NewEcosIOFactory(client.ctx, client.config, client.config.Credential.GetUserID(), bucketName)
	if err == nil && ret != nil {
		client.factoryPool.Add(bucketName, ret)
	}
	return ret, err
}

func (client *Client) GetVolumeOperator() *VolumeOperator {
	return &VolumeOperator{
		volumeID: client.config.Credential.GetUserID(),
		client:   client,
	}
}

func (client *Client) GetClusterOperator() Operator {
	return &ClusterOperator{client: client}
}
