package moon

import (
	"ecos/cloud/sun"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestRaft(t *testing.T) {
	basePath := "./ecos-data/db/moon"
	moons, rpcServers, err := createMoons(3, "", basePath)
	assert.NoError(t, err)

	// 先起三个节点
	var nodeInfos []*infos.NodeInfo
	for i := 0; i < 3; i++ {
		index := i
		nodeInfos = append(nodeInfos, moons[i].SelfInfo)
		go func() {
			err := rpcServers[index].Run()
			if err != nil {
				t.Errorf("Run rpcServer err: %v", err)
			}
		}()
		go moons[i].Run()
	}
	t.Cleanup(func() {
		for i := 0; i < 4; i++ {
			rpcServers[i].Stop()
			moons[i].Stop()
		}
		_ = os.RemoveAll(basePath)
	})

	// 等待选主
	leader := waitMoonsOK(moons)

	time.Sleep(2 * time.Second) // wait for NodeInfoStorage apply
	assertInfoStorageOK(t, len(moons), moons...)

	// Node4
	node4Info := infos.NewSelfInfo(0x04, "127.0.0.1", 32674)
	rpcServer4 := messenger.NewRpcServer(32674)
	moonConfig := DefaultConfig
	moonConfig.ClusterInfo = infos.ClusterInfo{
		Term:            0,
		LeaderInfo:      moons[leader-1].SelfInfo,
		NodesInfo:       nodeInfos,
		UpdateTimestamp: nil,
	}
	node4 := NewMoon(node4Info, moonConfig, rpcServer4, infos.NewMemoryNodeInfoStorage(),
		NewStorage(path.Join(basePath, "/raft", "/4")))
	moons = append(moons, node4)
	rpcServers = append(rpcServers, rpcServer4)
	go func() {
		err = rpcServer4.Run()
		if err != nil {
			t.Errorf("Run rpcServer err: %v", err)
		}
	}()

	// 启动 Node4
	// 集群提交增加节点请求
	go node4.Run()

	// 等待共识
	waitMoonsOK(moons)
	time.Sleep(3 * time.Second)

	// 判断集群是否达成共识
	assertInfoStorageOK(t, len(moons), moons...)
	info := moons[0].InfoStorage.ListAllNodeInfo()
	t.Log(info)
	for i := 1; i < 4; i++ {
		anotherInfo := moons[i].InfoStorage.ListAllNodeInfo()
		if diff := cmp.Diff(info, anotherInfo, protocmp.Transform()); diff != "" {
			t.Errorf("Node Info Not Equal")
		}
		t.Log(anotherInfo)
	}
	t.Log("Reach agreement success")
}

func assertInfoStorageOK(t *testing.T, nodeNum int, moons ...*Moon) {
	firstClusterInfo := moons[0].InfoStorage.GetClusterInfo(0)
	for _, moon := range moons {
		storage := moon.InfoStorage
		clusterInfo := storage.GetClusterInfo(0)
		if diff := cmp.Diff(firstClusterInfo, clusterInfo, protocmp.Transform()); diff != "" {
			t.Errorf("Group info not equal, diff: %v", diff)
		}
		assert.Equal(t, nodeNum, len(clusterInfo.NodesInfo),
			"node num in group info should same as real node num")
	}
}

func TestMoon_Register(t *testing.T) {
	dbBasePath := "./ecos-data/db/moon/"
	moonNum := 5

	port, sunRpc := messenger.NewRandomPortRpcServer()
	sun.NewSun(sunRpc)
	go func() {
		err := sunRpc.Run()
		if err != nil {
			t.Errorf("Run rpcServer err: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)

	moons, rpcServers, err := createMoons(moonNum, "127.0.0.1:"+strconv.FormatUint(port, 10), dbBasePath)
	assert.NoError(t, err)

	for i := 0; i < moonNum; i++ {
		go func(server *messenger.RpcServer) {
			err := server.Run()
			if err != nil {
				t.Errorf("Run rpcServer err: %v", err)
			}
		}(rpcServers[i])
		go moons[i].Run()
	}

	t.Cleanup(func() {
		sunRpc.Stop()
		for i := 0; i < moonNum; i++ {
			moons[i].Stop()
			rpcServers[i].Stop()
		}
		_ = os.RemoveAll(dbBasePath)
	})

	waitMoonsOK(moons)
	time.Sleep(4 * time.Second)
	assertInfoStorageOK(t, moonNum, moons...)
}

func waitMoonsOK(moons []*Moon) int {
	leader := -1
	for {
		ok := true
		for i := 0; i < len(moons); i++ {
			if moons[i].GetLeaderID() == 0 || len(moons[i].InfoStorage.ListAllNodeInfo()) != len(moons) {
				ok = false
			}
			leader = int(moons[i].GetLeaderID())
		}
		if !ok {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			logger.Infof("leader: %v", leader)
			break
		}
	}
	return leader
}

func createMoons(num int, sunAddr string, basePath string) ([]*Moon, []*messenger.RpcServer, error) {
	err := common.InitAndClearPath(basePath)
	if err != nil {
		return nil, nil, err
	}
	var infoStorages []infos.NodeInfoStorage
	var stableStorages []Storage
	var rpcServers []*messenger.RpcServer
	var moons []*Moon
	var nodeInfos []*infos.NodeInfo

	for i := 0; i < num; i++ {
		raftID := uint64(i + 1)
		//infoStorages = append(infoStorages,
		//	infos.NewStableNodeInfoStorage(path.Join(basePath, "/nodeInfo", strconv.Itoa(i+1))))
		infoStorages = append(infoStorages, infos.NewMemoryNodeInfoStorage())
		stableStorages = append(stableStorages, NewStorage(path.Join(basePath, "/raft", strconv.Itoa(i+1))))
		port, rpcServer := messenger.NewRandomPortRpcServer()
		rpcServers = append(rpcServers, rpcServer)
		nodeInfos = append(nodeInfos, infos.NewSelfInfo(raftID, "127.0.0.1", port))
	}

	moonConfig := DefaultConfig
	moonConfig.SunAddr = sunAddr
	moonConfig.ClusterInfo = infos.ClusterInfo{
		Term:            0,
		LeaderInfo:      nil,
		UpdateTimestamp: timestamp.Now(),
	}

	for i := 0; i < num; i++ {
		if sunAddr != "" {
			moons = append(moons, NewMoon(nodeInfos[i], moonConfig, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		} else {
			moonConfig.ClusterInfo.NodesInfo = nodeInfos
			moons = append(moons, NewMoon(nodeInfos[i], moonConfig, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		}
	}
	return moons, rpcServers, nil
}
