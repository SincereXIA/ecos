package alaya

import (
	"context"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/common"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/wxnacy/wgo/arrays"
	"google.golang.org/protobuf/types/known/timestamppb"
	"os"
	"path"
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
	os.Mkdir(nodeInfoDir, os.ModePerm)
	infoStorage := node.NewStableNodeInfoStorage(nodeInfoDir)
	defer infoStorage.Close()
	groupInfo := node.GroupInfo{
		GroupTerm: &node.Term{
			Term: 1,
		},
		LeaderInfo:      nil,
		NodesInfo:       []*node.NodeInfo{},
		UpdateTimestamp: timestamppb.Now(),
	}
	for i := 0; i < 9; i++ {
		info := node.NodeInfo{
			RaftId:   uint64(i) + 1,
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  uint64(32771 + i),
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
	os.Mkdir("./testMetaStorage/", os.ModePerm)
	for i := 0; i < 9; i++ { // for each node
		info := groupInfo.NodesInfo[i]
		dataBaseDir := "./testMetaStorage/" + strconv.FormatUint(info.RaftId, 10)
		metaStorage := NewStableMetaStorage(dataBaseDir)
		a := NewAlaya(info, infoStorage, metaStorage, &rpcServers[i], pipelines)
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

	for i := 0; i < 9; i++ { // test of whether the first node of pg is leader
		a := alayas[i]
		for _, p := range pipelines {
			if -1 == arrays.Contains(p.RaftId, a.NodeID) { // pass when node not in pipline
				continue
			}
			pgID := p.PgId
			if p.RaftId[0] == a.NodeID {
				a.PGRaftNode[pgID].raft.Status()
				assert.Equal(t, a.PGRaftNode[pgID].raft.Status().Lead, a.NodeID, "first node of pg is leader")
			}
		}
	}

	for i := 0; i < 9; i++ { // for each node
		a := alayas[i]
		a.printPipelineInfo()
	}

	a := alayas[pipelines[0].RaftId[0]-1]

	_, err := a.RecordObjectMeta(context.TODO(), &object.ObjectMeta{
		ObjId:      "/volume/bucket/testObj",
		Size:       100,
		UpdateTime: timestamppb.Now(),
		Blocks:     nil,
		PgId:       pipelines[0].PgId,
	})

	assert.NoError(t, err)
	time.Sleep(time.Second * 1)
	meta, err := a.MetaStorage.GetMeta("/volume/bucket/testObj")

	if err != nil {
		t.Errorf("get Meta fail, err:%v", err)
	}
	assert.Equal(t, uint64(100), meta.Size, "obj size")

	a2 := alayas[pipelines[0].RaftId[1]-1]
	meta2, err := a2.MetaStorage.GetMeta("/volume/bucket/testObj")

	assert.Equal(t, meta.UpdateTime, meta2.UpdateTime, "obj meta update time")

	for i := 0; i < 9; i++ { // for each node
		server := rpcServers[i]
		server.Stop()
		alaya := alayas[i]
		alaya.Stop()
	}

	os.RemoveAll("./testMetaStorage")
	os.RemoveAll("./NodeInfoStorage")

}

func TestAlaya_UpdatePipeline(t *testing.T) {
	var infoStorages []node.InfoStorage
	var nodeInfos []node.NodeInfo
	var rpcServers []*messenger.RpcServer
	term := uint64(1)

	for i := 0; i < 9; i++ {
		infoStorages = append(infoStorages, node.NewMemoryNodeInfoStorage())
		nodeInfos = append(nodeInfos, node.NodeInfo{
			RaftId:   uint64(i + 1),
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  uint64(32770 + i + 1),
			Capacity: 10,
		})
		rpcServers = append(rpcServers, messenger.NewRpcServer(uint64(32770+i+1)))
		infoStorages[i].UpdateNodeInfo(&nodeInfos[i])
		infoStorages[i].Commit(term)
		infoStorages[i].Apply()
	}

	// UP 6 Alaya
	var alayas []*Alaya
	for i := 0; i < 6; i++ {
		alayas = append(alayas, NewAlaya(&nodeInfos[i], infoStorages[i], NewMemoryMetaStorage(), rpcServers[i], nil))
		go rpcServers[i].Run()
		go alayas[i].Run()
	}
	time.Sleep(time.Second * 1)
	term = 2
	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			infoStorages[i].UpdateNodeInfo(&nodeInfos[j])
		}
		infoStorages[i].Commit(term)
	}
	for i := 0; i < 6; i++ {
		t.Logf("Apply new groupInfo for: %v", i+1)
		go infoStorages[i].Apply()
	}

	time.Sleep(time.Second * 5)

	for i := 0; i < 6; i++ { // for each node
		a := alayas[i]
		a.printPipelineInfo()
	}

	assertAlayasOK(t, alayas, pipeline.GenPipelines(infoStorages[0].GetGroupInfo(), 10, 3))

	// UP 3 Alaya
	for i := 6; i < 9; i++ {
		alayas = append(alayas, NewAlaya(&nodeInfos[i], infoStorages[i], NewMemoryMetaStorage(), rpcServers[i], nil))
		go rpcServers[i].Run()
		go alayas[i].Run()
	}
	time.Sleep(time.Second * 1)
	term = 3
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			infoStorages[i].UpdateNodeInfo(&nodeInfos[j])
		}
		infoStorages[i].Commit(term)
	}
	for i := 0; i < 9; i++ {
		t.Logf("Apply new groupInfo for: %v", i+1)
		go infoStorages[i].Apply()
	}
	time.Sleep(time.Second * 5)
	assertAlayasOK(t, alayas, pipeline.GenPipelines(infoStorages[0].GetGroupInfo(), 10, 3))
}

func assertAlayasOK(t *testing.T, alayas []*Alaya, pipelines []*pipeline.Pipeline) {
	for i := 0; i < len(alayas); i++ {
		a := alayas[i]
		for _, p := range pipelines {
			if -1 == arrays.Contains(p.RaftId, a.NodeID) { // pass when node not in pipline
				continue
			}
			pgID := p.PgId
			if p.RaftId[0] == a.NodeID {
				// test of whether the first node of pg is leader
				a.PGRaftNode[pgID].raft.Status()
				assert.Equal(t, a.PGRaftNode[pgID].raft.Status().Lead, a.NodeID, "first node of pg is leader")
			}
		}
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

func createMoons(num int, sunAddr string, basePath string) ([]*moon.Moon, []*messenger.RpcServer, error) {
	err := common.InitAndClearPath(basePath)
	if err != nil {
		return nil, nil, err
	}
	var infoStorages []node.InfoStorage
	var stableStorages []moon.Storage
	var rpcServers []*messenger.RpcServer
	var moons []*moon.Moon
	var nodeInfos []*node.NodeInfo

	for i := 0; i < num; i++ {
		raftID := uint64(i + 1)
		infoStorages = append(infoStorages,
			node.NewStableNodeInfoStorage(path.Join(basePath, "/nodeInfo", strconv.Itoa(i+1))))
		stableStorages = append(stableStorages, moon.NewStorage(path.Join(basePath, "/raft", strconv.Itoa(i+1))))
		rpcServers = append(rpcServers, messenger.NewRpcServer(32670+raftID))
		nodeInfos = append(nodeInfos, node.NewSelfInfo(raftID, "127.0.0.1", 32670+raftID))
	}

	for i := 0; i < num; i++ {
		if sunAddr != "" {
			moons = append(moons, moon.NewMoon(nodeInfos[i], sunAddr, nil, nil, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		} else {
			moons = append(moons, moon.NewMoon(nodeInfos[i], sunAddr, nil, nodeInfos, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		}
	}
	return moons, rpcServers, nil
}
