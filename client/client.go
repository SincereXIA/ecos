package client

import (
	"context"
	"ecos/client/config"
	agent "ecos/client/info-agent"
	"ecos/client/io"
	"ecos/cloud/rainbow"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"errors"
	lru "github.com/hashicorp/golang-lru"
	"path"
	"strconv"
	"strings"
	"sync"
)

type Client struct {
	ctx    context.Context
	cancel context.CancelFunc

	config    *config.ClientConfig
	InfoAgent *agent.InfoAgent

	factoryPool *lru.Cache

	mutex sync.RWMutex
}

const (
	// DefaultFactoryPoolSize is the default size of factory pool
	DefaultFactoryPoolSize = 10
)

func New(config *config.ClientConfig) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	clusterInfo := &infos.ClusterInfo{
		NodesInfo: []*infos.NodeInfo{
			{
				RaftId:  0,
				IpAddr:  config.NodeAddr,
				RpcPort: config.NodePort,
				State:   infos.NodeState_ONLINE,
			},
		},
	}
	infoAgent, err := agent.NewInfoAgent(ctx, clusterInfo, config.CloudAddr, config.CloudPort, config.ConnectType)

	if err != nil {
		cancel()
		return nil, err
	}
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
		InfoAgent:   infoAgent,
		factoryPool: lruPool,
	}, nil
}

func (client *Client) GetMoon() (moon.MoonClient, uint64, error) {
	for _, nodeInfo := range client.InfoAgent.GetCurClusterInfo().NodesInfo {
		if nodeInfo.State != infos.NodeState_ONLINE {
			continue
		}
		conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
		if err != nil {
			logger.Errorf("get rpc connection to node: %v fail: %v", nodeInfo.RaftId, err.Error())
			continue
		}
		moonClient := moon.NewMoonClient(conn)
		return moonClient, nodeInfo.RaftId, nil
	}
	return nil, 0, errno.ConnectionIssue
}

func (client *Client) getRainbow() (rainbow.RainbowClient, error) {
	conn, err := messenger.GetRpcConn(client.config.CloudAddr, client.config.CloudPort)
	if err != nil {
		return nil, err
	}
	return rainbow.NewRainbowClient(conn), nil
}

func (client *Client) ListObjects(_ context.Context, bucketName, prefix string) ([]*object.ObjectMeta, error) {
	userID := client.config.Credential.GetUserID()
	bucketID := infos.GenBucketID(userID, bucketName)
	info, err := client.InfoAgent.Get(infos.InfoType_BUCKET_INFO, bucketID)
	if err != nil {
		return nil, err
	}
	bucketInfo := info.BaseInfo().GetBucketInfo()
retry:
	clusterInfo := client.InfoAgent.GetCurClusterInfo()
	p := pipeline.GenMetaPipelines(clusterInfo)
	var result []*object.ObjectMeta
	for i := 1; int32(i) <= bucketInfo.Config.KeySlotNum; i++ {
		pgID := object.GenSlotPgID(bucketInfo.GetID(), int32(i), clusterInfo.MetaPgNum)
		nodeID := p[pgID-1].RaftId[0]
		info, err := client.InfoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(nodeID, 10))
		if err != nil {
			return nil, err
		}
		conn, err := messenger.GetRpcConnByNodeInfo(info.BaseInfo().GetNodeInfo())
		if err != nil {
			return nil, err
		}
		alayaClient := alaya.NewAlayaClient(conn)
		ctx, _ := alaya.SetTermToContext(client.ctx, clusterInfo.Term)
		reply, err := alayaClient.ListMeta(ctx, &alaya.ListMetaRequest{
			Prefix: path.Join(bucketInfo.GetID(), strconv.Itoa(i), prefix),
		})
		if err != nil {
			if strings.Contains(err.Error(), errno.TermNotMatch.Error()) {
				logger.Warningf("term not match, retry")
				err = client.InfoAgent.UpdateCurClusterInfo()
				if err != nil {
					return nil, err
				}
				result = nil
				goto retry
			}
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

func (client *Client) GetVolumeOperator() VolumeOperator {
	switch client.config.ConnectType {
	case config.ConnectCloud:
		return NewCloudVolumeOperator(client.ctx, client, client.config.Credential.GetUserID())
	}
	return NewEdgeVolumeOperator(client.ctx, client.config.Credential.GetUserID(), client)
}

func (client *Client) GetClusterOperator() Operator {
	return NewClusterOperator(client.ctx, client)
}
