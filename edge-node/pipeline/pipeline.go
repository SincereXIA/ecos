package pipeline

import (
	"ecos/edge-node/infos"
	"ecos/utils/logger"
	"github.com/sincerexia/gocrush"
)

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
