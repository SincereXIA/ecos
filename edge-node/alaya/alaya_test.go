package alaya

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestNewAlaya(t *testing.T) {
	nodeInfoDir := "./clusterInfoStorage"
	_ = common.InitAndClearPath(nodeInfoDir)
	//infoStorage := infos.NewStableNodeInfoStorage(nodeInfoDir)
	infoStorage := infos.NewMemoryNodeInfoStorage()
	defer infoStorage.Close()

	nodeNum := 9
	var rpcServers []messenger.RpcServer
	clusterInfo := infos.ClusterInfo{
		Term:            1,
		LeaderInfo:      nil,
		NodesInfo:       []*infos.NodeInfo{},
		UpdateTimestamp: timestamp.Now(),
	}
	for i := 0; i < nodeNum; i++ {
		port, server := messenger.NewRandomPortRpcServer()
		rpcServers = append(rpcServers, *server)
		info := infos.NodeInfo{
			RaftId:   uint64(i) + 1,
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  port,
			Capacity: 1,
		}
		_ = infoStorage.UpdateNodeInfo(&info, timestamp.Now())
		clusterInfo.NodesInfo = append(clusterInfo.NodesInfo, &info)
	}

	infoStorage.Commit(1)
	infoStorage.Apply()
	pipelines := pipeline.GenPipelines(&clusterInfo, 10, 3)

	var alayas []*Alaya
	_ = os.Mkdir("./testMetaStorage/", os.ModePerm)
	for i := 0; i < nodeNum; i++ { // for each node
		info := clusterInfo.NodesInfo[i]
		dataBaseDir := "./testMetaStorage/" + strconv.FormatUint(info.RaftId, 10)
		metaStorage := NewStableMetaStorage(dataBaseDir)
		a := NewAlaya(info, infoStorage, metaStorage, &rpcServers[i])
		alayas = append(alayas, a)
		server := rpcServers[i]
		go server.Run()
	}

	t.Cleanup(func() {
		for i := 0; i < nodeNum; i++ { // for each node
			alaya := alayas[i]
			alaya.Stop()
			server := rpcServers[i]
			server.Stop()
		}

		_ = os.RemoveAll("./testMetaStorage")
		_ = os.RemoveAll("./clusterInfoStorage")
	})

	t.Log("Alayas init done, start run")
	for i := 0; i < nodeNum; i++ {
		a := alayas[i]
		go a.Run()
	}
	waiteAllAlayaOK(alayas)
	assertAlayasOK(t, alayas, pipelines)

	for i := 0; i < nodeNum; i++ { // for each node
		a := alayas[i]
		a.PrintPipelineInfo()
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
	var infoStorages []infos.NodeInfoStorage
	var nodeInfos []infos.NodeInfo
	var rpcServers []*messenger.RpcServer
	var term uint64

	for i := 0; i < 9; i++ {
		port, rpcServer := messenger.NewRandomPortRpcServer()
		rpcServers = append(rpcServers, rpcServer)
		infoStorages = append(infoStorages, infos.NewMemoryNodeInfoStorage())
		nodeInfos = append(nodeInfos, infos.NodeInfo{
			RaftId:   uint64(i + 1),
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  port,
			Capacity: 10,
		})
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

	time.Sleep(time.Second * 1)
	for i := 0; i < 6; i++ {
		t.Logf("Apply new clusterInfo for: %v", i+1)
		go infoStorages[i].Apply()
	}

	waiteAllAlayaOK(alayas)

	for i := 0; i < 6; i++ { // for each node
		a := alayas[i]
		a.PrintPipelineInfo()
	}

	assertAlayasOK(t, alayas, pipeline.GenPipelines(infoStorages[0].GetClusterInfo(0), 10, 3))

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
		t.Logf("Apply new clusterInfo for: %v", i+1)
		go infoStorages[i].Apply()
	}
	waiteAllAlayaOK(alayas)
	assertAlayasOK(t, alayas, pipeline.GenPipelines(infoStorages[0].GetClusterInfo(0), 10, 3))
	for i := 0; i < 9; i++ { // for each node
		a := alayas[i]
		a.PrintPipelineInfo()
	}

	pipelines := pipeline.GenPipelines(infoStorages[0].GetClusterInfo(0), 10, 3)
	for _, p := range pipelines {
		t.Logf("PG: %v, id: %v, %v, %v", p.PgId, p.RaftId[0], p.RaftId[1], p.RaftId[2])
	}

	for i := 0; i < 9; i++ { // for each node
		server := rpcServers[i]
		server.Stop()
		alaya := alayas[i]
		alaya.Stop()
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

func waiteAllAlayaOK(alayas []*Alaya) {
	timer := time.After(60 * time.Second)
	for {
		select {
		case <-timer:
			logger.Warningf("Alayas not OK after time out")
			for _, alaya := range alayas {
				alaya.PrintPipelineInfo()
			}
			return
		default:
		}
		ok := true
		for id, alaya := range alayas {
			if !alaya.IsAllPipelinesOK() {
				logger.Warningf("Alaya %v not ok", id+1)
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(time.Millisecond * 200)
	}
}
