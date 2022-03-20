package alaya

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/timestamp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os"
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
	nodeInfoDir := "./NodeInfoStorage"
	_ = common.InitAndClearPath(nodeInfoDir)
	//infoStorage := node.NewStableNodeInfoStorage(nodeInfoDir)
	infoStorage := node.NewMemoryNodeInfoStorage()
	defer infoStorage.Close()
	groupInfo := node.GroupInfo{
		GroupTerm: &node.Term{
			Term: 1,
		},
		LeaderInfo:      nil,
		NodesInfo:       []*node.NodeInfo{},
		UpdateTimestamp: timestamp.Now(),
	}
	for i := 0; i < 9; i++ {
		info := node.NodeInfo{
			RaftId:   uint64(i) + 1,
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  uint64(32671 + i),
			Capacity: 1,
		}
		_ = infoStorage.UpdateNodeInfo(&info, timestamp.Now())
		groupInfo.NodesInfo = append(groupInfo.NodesInfo, &info)
	}
	infoStorage.Commit(1)
	infoStorage.Apply()
	pipelines := pipeline.GenPipelines(&groupInfo, 10, 3)
	var rpcServers []messenger.RpcServer
	for _, info := range groupInfo.NodesInfo {
		server := messenger.NewRpcServer(info.RpcPort)
		rpcServers = append(rpcServers, *server)
	}

	var alayas []*Alaya
	os.Mkdir("./testMetaStorage/", os.ModePerm)
	for i := 0; i < 9; i++ { // for each node
		info := groupInfo.NodesInfo[i]
		dataBaseDir := "./testMetaStorage/" + strconv.FormatUint(info.RaftId, 10)
		metaStorage := NewStableMetaStorage(dataBaseDir)
		a := NewAlaya(info, infoStorage, metaStorage, &rpcServers[i])
		alayas = append(alayas, a)
		server := rpcServers[i]
		go server.Run()
	}

	t.Cleanup(func() {
		for i := 0; i < 9; i++ { // for each node
			alaya := alayas[i]
			alaya.Stop()
			server := rpcServers[i]
			server.Stop()
		}

		_ = os.RemoveAll("./testMetaStorage")
		_ = os.RemoveAll("./NodeInfoStorage")
	})

	t.Log("Alayas init done, start run")
	for i := 0; i < 9; i++ {
		a := alayas[i]
		go a.Run()
	}
	time.Sleep(time.Second * 8)
	assertAlayasOK(t, alayas, pipelines)

	for i := 0; i < 9; i++ { // for each node
		a := alayas[i]
		a.printPipelineInfo()
	}

	a := alayas[pipelines[0].RaftId[0]-1]

	_, err := a.RecordObjectMeta(context.TODO(), &object.ObjectMeta{
		ObjId:      "/volume/bucket/testObj",
		ObjSize:    100,
		UpdateTime: timestamp.Now(),
		Blocks:     nil,
		PgId:       pipelines[0].PgId,
	})

	assert.NoError(t, err)
	time.Sleep(time.Second * 1)
	meta, err := a.MetaStorage.GetMeta("/volume/bucket/testObj")

	if err != nil {
		t.Errorf("get Meta fail, err:%v", err)
	}
	assert.Equal(t, uint64(100), meta.ObjSize, "obj size")

	a2 := alayas[pipelines[0].RaftId[1]-1]
	meta2, err := a2.MetaStorage.GetMeta("/volume/bucket/testObj")

	assert.Equal(t, meta.UpdateTime, meta2.UpdateTime, "obj meta update time")

}

func TestAlaya_UpdatePipeline(t *testing.T) {
	var infoStorages []node.InfoStorage
	var nodeInfos []node.NodeInfo
	var rpcServers []*messenger.RpcServer
	var term uint64

	for i := 0; i < 9; i++ {
		infoStorages = append(infoStorages, node.NewMemoryNodeInfoStorage())
		nodeInfos = append(nodeInfos, node.NodeInfo{
			RaftId:   uint64(i + 1),
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  uint64(32670 + i + 1),
			Capacity: 10,
		})
		rpcServers = append(rpcServers, messenger.NewRpcServer(uint64(32670+i+1)))
	}

	// UP 6 Alaya
	var alayas []*Alaya
	term = 2
	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			_ = infoStorages[i].UpdateNodeInfo(&nodeInfos[j], timestamp.Now())
		}
		infoStorages[i].Commit(term)
		alayas = append(alayas, NewAlaya(&nodeInfos[i], infoStorages[i], NewMemoryMetaStorage(), rpcServers[i]))
		go func(server *messenger.RpcServer) {
			err := server.Run()
			if err != nil {
				t.Errorf("Run rpc server at port: %v fail", nodeInfos[i].RpcPort)
			}
		}(rpcServers[i])
		go alayas[i].Run()
	}

	t.Cleanup(func() {
		for i := 0; i < 9; i++ { // for each node
			server := rpcServers[i]
			server.Stop()
			alaya := alayas[i]
			alaya.Stop()
		}
	})

	time.Sleep(time.Second * 1)
	for i := 0; i < 6; i++ {
		t.Logf("Apply new groupInfo for: %v", i+1)
		go infoStorages[i].Apply()
	}

	time.Sleep(time.Second * 15)

	for i := 0; i < 6; i++ { // for each node
		a := alayas[i]
		a.printPipelineInfo()
	}

	assertAlayasOK(t, alayas, pipeline.GenPipelines(infoStorages[0].GetGroupInfo(0), 10, 3))

	for i := 6; i < 9; i++ {
		for j := 0; j < 6; j++ {
			_ = infoStorages[i].UpdateNodeInfo(&nodeInfos[j], timestamp.Now())
		}
		infoStorages[i].Commit(term)
		infoStorages[i].Apply()
	}
	// UP 3 Alaya
	for i := 6; i < 9; i++ {
		alayas = append(alayas, NewAlaya(&nodeInfos[i], infoStorages[i], NewMemoryMetaStorage(), rpcServers[i]))
		go func(server *messenger.RpcServer) {
			err := server.Run()
			if err != nil {
				t.Errorf("Run rpc server at port: %v fail", nodeInfos[i].RpcPort)
			}
		}(rpcServers[i])
		go alayas[i].Run()
	}
	time.Sleep(time.Second * 1)
	term = 3
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			_ = infoStorages[i].UpdateNodeInfo(&nodeInfos[j], timestamp.Now())
		}
		infoStorages[i].Commit(term)
	}
	for i := 0; i < 9; i++ {
		t.Logf("Apply new groupInfo for: %v", i+1)
		go infoStorages[i].Apply()
	}
	time.Sleep(time.Second * 15)
	assertAlayasOK(t, alayas, pipeline.GenPipelines(infoStorages[0].GetGroupInfo(0), 10, 3))
	for i := 0; i < 9; i++ { // for each node
		a := alayas[i]
		a.printPipelineInfo()
	}

	pipelines := pipeline.GenPipelines(infoStorages[0].GetGroupInfo(0), 10, 3)
	for _, p := range pipelines {
		t.Logf("PG: %v, id: %v, %v, %v", p.PgId, p.RaftId[0], p.RaftId[1], p.RaftId[2])
	}
}

func assertAlayasOK(t *testing.T, alayas []*Alaya, pipelines []*pipeline.Pipeline) {
	// 判断 每个 pg 第一个 节点是否为 leader
	for _, p := range pipelines {
		leaderID := p.RaftId[0]
		pgID := p.PgId
		a := alayas[leaderID-1]
		assert.Equal(t, leaderID, a.getRaftNode(pgID).raft.Status().Lead)
	}
	// 判断 每个 alaya 的每个 raft node 是否都成功加入 PG
	for _, a := range alayas {
		a.PGRaftNode.Range(func(key, value interface{}) bool {
			raftNode := value.(*Raft)
			assert.NotZero(t, raftNode.raft.Status().Lead)
			return true
		})
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
