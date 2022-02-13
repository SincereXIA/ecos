package pipline

import (
	"ecos/edge-node/node"
	"math/rand"
)

func GenPiplines(groupInfo *node.GroupInfo, pgNum uint64, groupSize uint64) []*Pipline {
	//TODO: Gen piplines by crush
	var rs []*Pipline
	for i := uint64(0); i < pgNum; i++ {
		ids := make([]uint64, groupSize)
		for j := uint64(0); j < groupSize; j++ {
			ok := false
			for !ok {
				r := rand.Uint64() % uint64(len(groupInfo.NodesInfo))
				ids[j] = groupInfo.NodesInfo[r].RaftId
				ok = true
				for index, id := range ids {
					if uint64(index) >= j {
						break
					}
					if id == ids[j] {
						ok = false
						break
					}
				}
			}
		}
		pip := Pipline{
			PgId:     i,
			RaftId:   ids,
			SyncType: 0,
		}
		rs = append(rs, &pip)
	}
	return rs
}
