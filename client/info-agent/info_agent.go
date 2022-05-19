package info_agent

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"sync"
)

type InfoAgent struct {
	ctx    context.Context
	cancel context.CancelFunc

	nodeAddr string
	nodePort uint64

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
	// no cache, start request
	nodeInfo := agent.currentClusterInfo.NodesInfo[0]
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return infos.InvalidInfo{}, err
	}
	m := moon.NewMoonClient(conn)
	req := &moon.GetInfoRequest{
		InfoId:   id,
		InfoType: infoType,
	}
	result, err := m.GetInfo(agent.ctx, req)
	if err != nil {
		return infos.InvalidInfo{}, err
	}
	_ = agent.Update(result.GetBaseInfo())
	return result.GetBaseInfo(), err
}

// GetCurClusterInfo Returns current ClusterInfo
func (agent *InfoAgent) GetCurClusterInfo() infos.ClusterInfo {
	agent.mutex.RLock()
	defer agent.mutex.RUnlock()
	return *agent.currentClusterInfo
}

// UpdateCurClusterInfo Updates current ClusterInfo
func (agent *InfoAgent) UpdateCurClusterInfo() error {
	agent.mutex.Lock()
	defer agent.mutex.Unlock()
	conn, err := messenger.GetRpcConn(agent.nodeAddr, agent.nodePort)
	if err != nil {
		logger.Errorf("connect to node failed: %s", err.Error())
		return err
	}
	watcherClient := watcher.NewWatcherClient(conn)
	reply, err := watcherClient.GetClusterInfo(agent.ctx,
		&watcher.GetClusterInfoRequest{Term: 0})
	if err != nil {
		logger.Errorf("get cluster info failed: %s", err.Error())
		return err
	}
	agent.currentClusterInfo = reply.GetClusterInfo()
	return nil
}

func NewInfoAgent(ctx context.Context, nodeAddr string, nodePort uint64) (*InfoAgent, error) {
	conn, err := messenger.GetRpcConn(nodeAddr, nodePort)
	if err != nil {
		logger.Errorf("connect to node failed: %s", err.Error())
		return nil, errno.ConnectionIssue
	}
	watcherClient := watcher.NewWatcherClient(conn)
	reply, err := watcherClient.GetClusterInfo(ctx,
		&watcher.GetClusterInfoRequest{Term: 0})
	if err != nil {
		logger.Errorf("get cluster info failed: %s", err.Error())
		return nil, errno.ClusterInfoErr
	}
	clusterInfo := reply.GetClusterInfo()
	builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
	ctx, cancel := context.WithCancel(ctx)
	return &InfoAgent{
		ctx:                ctx,
		cancel:             cancel,
		mutex:              sync.RWMutex{},
		currentClusterInfo: clusterInfo,
		nodeAddr:           nodeAddr,
		nodePort:           nodePort,
		StorageRegister:    builder.GetStorageRegister(),
	}, nil
}
