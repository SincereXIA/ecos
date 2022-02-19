package alaya

import (
	"ecos/edge-node/node"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
	"time"
)
import "github.com/sincerexia/gocrush"

const (
	ROOT        = 0
	DATA_CENTER = 1
	RACK        = 2
	NODE        = 3
	DISK        = 4
)

func TestNewAlaya(t *testing.T) {
	infoStorage := node.NewMemoryNodeInfoStorage()
	groupInfo := node.GroupInfo{
		Term:            1,
		LeaderInfo:      nil,
		NodesInfo:       []*node.NodeInfo{},
		UpdateTimestamp: uint64(time.Now().Unix()),
	}
	for i := 0; i < 9; i++ {
		info := node.NodeInfo{
			RaftId:   uint64(i) + 1,
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  uint64(32671 + i),
			Capacity: 1,
		}
		infoStorage.UpdateNodeInfo(&info)
		groupInfo.NodesInfo = append(groupInfo.NodesInfo, &info)
	}
	pipelines := pipeline.GenPipelines(&groupInfo, 9, 3)
	var rpcServers []messenger.RpcServer
	for _, info := range groupInfo.NodesInfo {
		server := messenger.NewRpcServer(info.RpcPort)
		rpcServers = append(rpcServers, *server)
	}
	var alayas []*Alaya
	for i := 0; i < 9; i++ { // for each node
		info := groupInfo.NodesInfo[i]
		a := NewAlaya(info, infoStorage, &rpcServers[i], pipelines)
		alayas = append(alayas, a)
		server := rpcServers[i]
		go server.Run()
	}
	t.Log("Alayas init done, start run")
	for i := 0; i < 9; i++ {
		a := alayas[i]
		a.Run()
	}
	time.Sleep(time.Second * 5)

	for i := 0; i < 9; i++ { // for each node
		a := alayas[i]
		a.printPiplineInfo()
	}
}

func TestAlaya_RecordObjectMeta(t *testing.T) {
	tree := makeStrawTree()
	nodes := gocrush.Select(tree, 868, 3, NODE, nil)
	for _, n := range nodes {
		t.Logf("node: %v", n.GetId())
	}
	checkUnique(t, nodes)
	nodes = gocrush.Select(tree, 11, 3, NODE, nil)
	for _, n := range nodes {
		t.Logf("node: %v", n.GetId())
	}
	checkUnique(t, nodes)
}

func checkUnique(t *testing.T, nodes []gocrush.Node) {
	m := make(map[string]int)
	for _, n := range nodes {
		m[n.GetId()] = 1
	}
	assert.Equal(t, len(m), len(nodes))
}

func makeStrawTree() *gocrush.TestingNode {

	var parent = new(gocrush.TestingNode)
	parent.Id = "ROOT"
	parent.Type = ROOT
	parent.Weight = 0
	parent.Children = make([]gocrush.Node, 50)
	for dc := 0; dc < 50; dc++ {
		var node = new(gocrush.TestingNode)
		node.Parent = parent
		node.Weight = 10
		node.Type = NODE
		node.Id = parent.Id + ":NODE" + strconv.Itoa(dc)

		parent.Children[dc] = node
		node.Selector = gocrush.NewStrawSelector(node)
	}
	parent.Selector = gocrush.NewStrawSelector(parent)
	return parent
}
