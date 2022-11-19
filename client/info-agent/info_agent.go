package info_agent

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	moon2 "ecos/shared/moon"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"strconv"
	"sync"
)

type InfoAgent struct {
	ctx    context.Context
	cancel context.CancelFunc

	cloudAddr string
	cloudPort uint64

	connectType int

	mutex              sync.RWMutex
	currentClusterInfo *infos.ClusterInfo
	*infos.StorageRegister
}

func (agent *InfoAgent) GetStorage(infoType infos.InfoType) infos.Storage {
	return agent.StorageRegister.GetStorage(infoType)
}

// Update will store the Information into corresponding Storage,
// the Storage must Register before.
func (agent *InfoAgent) Update(info infos.Information) error {
	return agent.StorageRegister.Update(info)
}

// Delete will delete the Information from corresponding Storage
// by id, the Storage must Register before.
func (agent *InfoAgent) Delete(infoType infos.InfoType, id string) error {
	return agent.StorageRegister.Delete(infoType, id)
}

// Get will return the Information requested from corresponding Storage
// by id, the Storage must Register before.
func (agent *InfoAgent) Get(infoType infos.InfoType, id string) (infos.Information, error) {
	info, err := agent.StorageRegister.Get(infoType, id)
	if err == nil {
		return info, err
	}
	switch agent.connectType {
	case config.ConnectEdge:
		return agent.GetInfoByEdge(infoType, id)
	case config.ConnectCloud:
		return agent.GetInfoByCloud(infoType, id)
	}
	return infos.InvalidInfo{}, errno.ConnectionIssue
}

func (agent *InfoAgent) GetInfoByEdge(infoType infos.InfoType, id string) (infos.Information, error) {
	for _, nodeInfo := range agent.currentClusterInfo.NodesInfo {
		conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
		if err != nil {
			logger.Warningf("create rpc connect to node: %v, err: %v", nodeInfo.RaftId, err.Error())
			continue
		}
		m := moon2.NewMoonClient(conn)
		req := &moon2.GetInfoRequest{
			InfoId:   id,
			InfoType: infoType,
		}
		result, err := m.GetInfo(agent.ctx, req)
		if err != nil {
			logger.Warningf("get info from node: %v, err: %v", nodeInfo.RaftId, err.Error())
			continue
		}
		_ = agent.Update(result.GetBaseInfo())
		return result.GetBaseInfo(), err
	}
	return infos.InvalidInfo{}, errno.ConnectionIssue
}

func (agent *InfoAgent) GetInfoByCloud(infoType infos.InfoType, id string) (infos.Information, error) {
	conn, err := messenger.GetRpcConn(agent.cloudAddr, agent.cloudPort)
	if err != nil {
		logger.Errorf("connect to cloud: %v:%v failed: %s", agent.cloudAddr, agent.cloudPort, err.Error())
		return infos.InvalidInfo{}, errno.ConnectionIssue
	}

	m := moon2.NewMoonClient(conn)
	req := &moon2.GetInfoRequest{
		InfoId:   id,
		InfoType: infoType,
	}
	result, err := m.GetInfo(agent.ctx, req)
	if err != nil {
		logger.Warningf("get info from cloud err: %v", err.Error())
		return infos.InvalidInfo{}, errno.ConnectionIssue
	}
	_ = agent.Update(result.GetBaseInfo())
	return result.GetBaseInfo(), err
}

// GetCurClusterInfo Returns current ClusterInfo
func (agent *InfoAgent) GetCurClusterInfo() infos.ClusterInfo {
	if agent.currentClusterInfo == nil || agent.currentClusterInfo.Term == 0 {
		_ = agent.UpdateCurClusterInfo()
	}
	agent.mutex.RLock()
	defer agent.mutex.RUnlock()
	return *agent.currentClusterInfo
}

// UpdateCurClusterInfo Updates current ClusterInfo
func (agent *InfoAgent) UpdateCurClusterInfo() error {
	agent.mutex.Lock()
	defer agent.mutex.Unlock()
	switch agent.connectType {
	case config.ConnectCloud:
		info, err := agent.GetInfoByCloud(infos.InfoType_CLUSTER_INFO, strconv.Itoa(0))
		if err != nil {
			return err
		}
		agent.currentClusterInfo = info.BaseInfo().GetClusterInfo()
		return nil
	default:
	}

	for _, nodeInfo := range agent.currentClusterInfo.NodesInfo {
		conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
		if err != nil {
			logger.Errorf("connect to node failed: %s", err.Error())
			continue
		}
		watcherClient := watcher.NewWatcherClient(conn)
		reply, err := watcherClient.GetClusterInfo(agent.ctx,
			&watcher.GetClusterInfoRequest{Term: 0})
		if err != nil {
			logger.Errorf("get cluster info failed: %s", err.Error())
			continue
		}
		agent.currentClusterInfo = reply.GetClusterInfo()
		return nil
	}
	return nil
}

func NewInfoAgent(ctx context.Context, clusterInfo *infos.ClusterInfo, cloudAddr string,
	cloudPort uint64, connectType int) (*InfoAgent, error) {
	builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
	ctx, cancel := context.WithCancel(ctx)
	return &InfoAgent{
		ctx:                ctx,
		cancel:             cancel,
		mutex:              sync.RWMutex{},
		currentClusterInfo: clusterInfo,

		cloudAddr:   cloudAddr,
		cloudPort:   cloudPort,
		connectType: connectType,

		StorageRegister: builder.GetStorageRegister(),
	}, nil
}
