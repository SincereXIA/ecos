package pipeline

import (
	"ecos/edge-node/infos"
	"ecos/utils/logger"
	"github.com/sincerexia/gocrush"
)

type ClusterPipelines struct {
	Term           uint64
	MetaPipelines  []*Pipeline
	BlockPipelines []*Pipeline
}

func (cp *ClusterPipelines) GetMetaPG(pgID uint64) []uint64 {
	return cp.MetaPipelines[pgID-1].RaftId
}

func (cp *ClusterPipelines) GetBlockPG(pgID uint64) []uint64 {
	return cp.BlockPipelines[pgID-1].RaftId
}

func NewClusterPipelines(info *infos.ClusterInfo) (*ClusterPipelines, error) {
	metaPipelines := GenMetaPipelines(*info)
	blockPipelines := GenBlockPipelines(*info)
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
	for _, info := range clusterInfo.NodesInfo {
		rootNode.Children = append(rootNode.Children,
			&infos.EcosNode{
				NodeInfo: info,
				Type:     infos.LeafNodeType,
				Failed:   false,
				Root:     rootNode,
			},
		)
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
			PgId:     i,
			RaftId:   ids,
			SyncType: 0,
		}
		rs = append(rs, &pipeline)
	}
	return rs
}
