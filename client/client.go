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
)

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc

	config      *config.ClientConfig
	clusterInfo *infos.ClusterInfo
	infoAgent   *info_agent.InfoAgent

	factoryPool *lru.Cache
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
		switch value.(type) {
		case *io.EcosIOFactory:
			err := value.(*io.EcosIOFactory).AbortAllMultipartUploadJob()
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

func (client *Client) ListObjects(_ context.Context, bucketName string) ([]*object.ObjectMeta, error) {
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
		reply, err := alayaClient.ListMeta(client.ctx, &alaya.ListMetaRequest{
			Prefix: path.Join(bucketInfo.GetID(), strconv.Itoa(i)),
		})
		if err != nil {
			return nil, err
		}
		result = append(result, reply.Metas...)
	}
	return result, nil
}

func (client *Client) GetIOFactory(bucketName string) *io.EcosIOFactory {
	if ret, ok := client.factoryPool.Get(bucketName); ok {
		return ret.(*io.EcosIOFactory)
	}
	ret := io.NewEcosIOFactory(client.config, client.config.Credential.GetUserID(), bucketName)
	client.factoryPool.Add(bucketName, ret)
	return ret
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
