package pipeline

import (
	"ecos/edge-node/infos"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"github.com/sincerexia/gocrush"
	"strconv"
)

type ClusterPipelines struct {
	Term           uint64
	MetaPipelines  []*Pipeline
	BlockPipelines []*Pipeline
}

func (cp *ClusterPipelines) GetMetaPG(pgID uint64) []uint64 {
	return cp.MetaPipelines[pgID-1].RaftId
}

func (cp *ClusterPipelines) GetMetaPGNodeID(pgID uint64) []string {
	NodeIDs := make([]string, 0, len(cp.MetaPipelines[pgID-1].RaftId))
	for _, id := range cp.MetaPipelines[pgID-1].RaftId {
		NodeIDs = append(NodeIDs, strconv.FormatUint(id, 10))
	}
	return NodeIDs
}

func (cp *ClusterPipelines) GetBlockPG(pgID uint64) []uint64 {
	return cp.BlockPipelines[pgID-1].RaftId
}

func (cp *ClusterPipelines) GetBlockPGNodeID(pgID uint64) []string {
	NodeIDs := make([]string, 0, len(cp.BlockPipelines[pgID-1].RaftId))
	for _, id := range cp.BlockPipelines[pgID-1].RaftId {
		NodeIDs = append(NodeIDs, strconv.FormatUint(id, 10))
	}
	return NodeIDs
}

func (cp *ClusterPipelines) GetMetaPipeline(pgID uint64) *Pipeline {
	return cp.MetaPipelines[pgID-1]
}

func (cp *ClusterPipelines) GetBlockPipeline(pgID uint64) *Pipeline {
	return cp.BlockPipelines[pgID-1]
}

func NewClusterPipelines(info infos.ClusterInfo) (*ClusterPipelines, error) {
	metaPipelines := GenMetaPipelines(info)
	blockPipelines := GenBlockPipelines(info)
	if len(metaPipelines) == 0 || len(blockPipelines) == 0 {
		return nil, errno.ClusterNotOK
	}
	return &ClusterPipelines{
		Term:           info.Term,
		MetaPipelines:  metaPipelines,
		BlockPipelines: blockPipelines,
	}, nil
}

func GenMetaPipelines(clusterInfo infos.ClusterInfo) []*Pipeline {
	return GenPipelines(clusterInfo, uint64(clusterInfo.MetaPgNum), uint64(clusterInfo.MetaPgSize))
}

func GenBlockPipelines(clusterInfo infos.ClusterInfo) []*Pipeline {
	return GenPipelines(clusterInfo, uint64(clusterInfo.BlockPgNum), uint64(clusterInfo.BlockPgSize))
}

func GenPipelines(clusterInfo infos.ClusterInfo, pgNum uint64, groupSize uint64) []*Pipeline {
	var rs []*Pipeline
	rootNode := infos.NewRootNode()
	rootNode.Root = rootNode
	healthNodeNum := 0
	for _, info := range clusterInfo.NodesInfo {
		if info.State == infos.NodeState_OFFLINE {
			continue
		}
		healthNodeNum += 1
		rootNode.Children = append(rootNode.Children,
			&infos.EcosNode{
				NodeInfo: info,
				Type:     infos.LeafNodeType,
				Failed:   false,
				Root:     rootNode,
			},
		)
	}
	if uint64(healthNodeNum) < groupSize {
		return nil
	}
	rootNode.Selector = gocrush.NewStrawSelector(rootNode)
	for i := uint64(1); i <= pgNum; i++ {
		nodes := gocrush.Select(rootNode, int64(i), int(groupSize), infos.LeafNodeType, nil)
		var ids []uint64
		for _, n := range nodes {
			raftId := n.(*infos.EcosNode).GetRaftId()
			if raftId == 0 {
				logger.Errorf("Unable to get RaftId! %#v", n)
			}
			ids = append(ids, raftId)
		}
		pipeline := Pipeline{
			PgId:        i,
			RaftId:      ids,
			SyncType:    0,
			ClusterTerm: clusterInfo.Term,
		}
		rs = append(rs, &pipeline)
	}
	return rs
}
