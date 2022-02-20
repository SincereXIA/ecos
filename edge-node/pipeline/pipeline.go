package pipeline

import (
	"ecos/edge-node/node"
	"ecos/utils/logger"
	"github.com/sincerexia/gocrush"
)

func GenPipelines(groupInfo *node.GroupInfo, pgNum uint64, groupSize uint64) []*Pipeline {
	var rs []*Pipeline
	//for i := uint64(0); i < pgNum; i++ {
	//	ids := make([]uint64, groupSize)
	//	for j := uint64(0); j < groupSize; j++ {
	//		ok := false
	//		for !ok {
	//			r := rand.Uint64() % uint64(len(groupInfo.NodesInfo))
	//			ids[j] = groupInfo.NodesInfo[r].RaftId
	//			ok = true
	//			for index, id := range ids {
	//				if uint64(index) >= j {
	//					break
	//				}
	//				if id == ids[j] {
	//					ok = false
	//					break
	//				}
	//			}
	//		}
	//	}
	//	pip := Pipline{
	//		PgId:     i,
	//		RaftId:   ids,
	//		SyncType: 0,
	//	}
	//	rs = append(rs, &pip)
	//}
	rootNode := node.NewRootNode()
	rootNode.Root = rootNode
	for _, info := range groupInfo.NodesInfo {
		rootNode.Children = append(rootNode.Children,
			&node.EcosNode{
				NodeInfo: info,
				Type:     node.LeafNodeType,
				Failed:   false,
				Root:     rootNode,
			},
		)
	}
	rootNode.Selector = gocrush.NewStrawSelector(rootNode)
	for i := uint64(1); i <= pgNum; i++ {
		nodes := gocrush.Select(rootNode, int64(i), int(groupSize), node.LeafNodeType, nil)
		var ids []uint64
		for _, n := range nodes {
			raftId := n.(*node.EcosNode).GetRaftId()
			//ref := reflect.ValueOf(rootNode)
			//method := ref.MethodByName("GetRaftId")
			//if method.IsValid() {
			//	ret := method.Call(make([]reflect.Value, 0))
			//	raftId := ret[0].Uint()
			//	ids = append(ids, raftId)
			//} else {
			//	logger.Errorf("Unable to get RaftId! %#v", n)
			//	ids = append(ids, 0)
			//}
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
