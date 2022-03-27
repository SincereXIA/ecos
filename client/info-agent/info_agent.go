package info_agent

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/messenger"
)

type InfoAgent struct {
	ctx                context.Context
	currentClusterInfo *infos.ClusterInfo
	*infos.StorageRegister

	cancel context.CancelFunc
}

func (agent *InfoAgent) GetStorage(infoType infos.InfoType) infos.Storage {
	return agent.StorageRegister.GetStorage(infoType)
}

// Update will store the Information into corresponding Storage,
// the Storage must Register before.
func (agent *InfoAgent) Update(id string, info infos.Information) error {
	return agent.StorageRegister.Update(id, info)
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
		return infos.Information{}, err
	}
	m := moon.NewMoonClient(conn)
	req := &moon.GetInfoRequest{
		InfoId:   id,
		InfoType: infoType,
	}
	result, err := m.GetInfo(agent.ctx, req)
	if err != nil {
		return infos.Information{}, err
	}
	info, _ = infos.BaseInfoToInformation(*result.BaseInfo)
	_ = agent.Update(id, info)
	return info, err
}

func NewInfoAgent(ctx context.Context, currentClusterInfo *infos.ClusterInfo) *InfoAgent {
	builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
	ctx, cancel := context.WithCancel(ctx)
	return &InfoAgent{
		ctx:                ctx,
		cancel:             cancel,
		currentClusterInfo: currentClusterInfo,
		StorageRegister:    builder.GetStorageRegister(),
	}
}
