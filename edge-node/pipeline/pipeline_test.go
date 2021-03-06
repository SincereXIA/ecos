package pipeline

import (
	"ecos/edge-node/infos"
	"ecos/utils/timestamp"
	"github.com/stretchr/testify/assert"
	"reflect"
	"strconv"
	"testing"
)

func TestGenPipelines(t *testing.T) {
	group := genClusterInfo()
	pipelines := GenPipelines(*group, 10, 3)
	for _, pipeline := range pipelines {
		t.Logf("PG: %v, id: %v, %v, %v", pipeline.PgId, pipeline.RaftId[0], pipeline.RaftId[1], pipeline.RaftId[2])
	}
	pipelines2 := GenPipelines(*group, 10, 3)
	for _, p := range pipelines2 {
		t.Logf("PG: %v, id: %v, %v, %v", p.PgId, p.RaftId[0], p.RaftId[1], p.RaftId[2])
	}
	assert.True(t, reflect.DeepEqual(pipelines, pipelines2))
}

func genClusterInfo() *infos.ClusterInfo {
	group := infos.ClusterInfo{
		Term:            0,
		LeaderInfo:      nil,
		NodesInfo:       []*infos.NodeInfo{},
		UpdateTimestamp: timestamp.Now(),
	}
	for i := 1; i <= 20; i++ {
		group.NodesInfo = append(group.NodesInfo, &infos.NodeInfo{
			RaftId:   uint64(i),
			Uuid:     strconv.Itoa(i) + "test",
			IpAddr:   "127.0.0.1",
			RpcPort:  0,
			Capacity: 10,
		})
	}
	return &group
}
