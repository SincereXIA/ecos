package pipline

import (
	"ecos/edge-node/node"
	"github.com/google/uuid"
	"testing"
)

func TestGenPiplines(t *testing.T) {
	group := genGroupInfo()
	piplines := GenPiplines(group, 100, 3)
	for _, pipline := range piplines {
		t.Logf("PG: %v, id: %v, %v, %v", pipline.PgId, pipline.RaftId[0], pipline.RaftId[1], pipline.RaftId[2])
	}
}

func genGroupInfo() *node.GroupInfo {
	group := node.GroupInfo{
		Term:            0,
		LeaderInfo:      nil,
		NodesInfo:       []*node.NodeInfo{},
		UpdateTimestamp: 0,
	}
	for i := 0; i < 20; i++ {
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
