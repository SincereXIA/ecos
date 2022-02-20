package pipeline

import (
	"ecos/edge-node/node"
	"github.com/google/uuid"
	"testing"
)

func TestGenPipelines(t *testing.T) {
	group := genGroupInfo()
	pipelines := GenPipelines(group, 100, 3)
	for _, pipeline := range pipelines {
		t.Logf("PG: %v, id: %v, %v, %v", pipeline.PgId, pipeline.RaftId[0], pipeline.RaftId[1], pipeline.RaftId[2])
	}
}

func genGroupInfo() *node.GroupInfo {
	group := node.GroupInfo{
		Term:            0,
		LeaderInfo:      nil,
		NodesInfo:       []*node.NodeInfo{},
		UpdateTimestamp: 0,
	}
	for i := 1; i <= 20; i++ {
		group.NodesInfo = append(group.NodesInfo, &node.NodeInfo{
			RaftId:   uint64(i),
			Uuid:     uuid.New().String(),
			IpAddr:   "",
			RpcPort:  0,
			Capacity: 0,
		})
	}
	return &group
}
