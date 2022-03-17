package moon

import (
	"ecos/cloud/sun"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/utils/common"
	"ecos/utils/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/raft/v3"
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
	var nodeInfos []*node.NodeInfo
	for i := 0; i < 3; i++ {
		index := i
		nodeInfos = append(nodeInfos, moons[i].selfInfo)
		go func() {
			err := rpcServers[index].Run()
			if err != nil {
				t.Errorf("Run rpcServer err: %v", err)
			}
		}()
		go moons[i].Run()
	}

	// 等待选主
	leader := -1
	for {
		for i := 0; i < 3; i++ {
			if moons[i].raft != nil && raft.StateLeader == moons[i].raft.Status().RaftState {
				leader = i
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if leader >= 0 {
			t.Logf("leader: %v", leader+1)
			break
		}
	}

	// Node4
	node4Info := node.NewSelfInfo(0x04, "127.0.0.1", 32674)
	rpcServer4 := messenger.NewRpcServer(32674)
	moonConfig := DefaultConfig
	moonConfig.GroupInfo = node.GroupInfo{
		GroupTerm:       &node.Term{Term: 0},
		LeaderInfo:      moons[leader].selfInfo,
		NodesInfo:       nodeInfos,
		UpdateTimestamp: nil,
	}
	node4 := NewMoon(node4Info, &moonConfig, rpcServer4, node.NewMemoryNodeInfoStorage(),
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
	time.Sleep(3 * time.Second)

	// 判断集群是否达成共识
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

	for i := 0; i < 4; i++ {
		rpcServers[i].Stop()
		moons[i].Stop()
	}

	_ = os.RemoveAll(basePath)
}

func TestMoon_Register(t *testing.T) {
	dbBasePath := "./ecos-data/db/moon/"
	moonNum := 5

	defer func(path string) {
		_ = os.RemoveAll(path)
	}(dbBasePath)

	sunRpc := messenger.NewRpcServer(3260)
	sun.NewSun(sunRpc)
	go func() {
		err := sunRpc.Run()
		if err != nil {
			t.Errorf("Run rpcServer err: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)

	moons, rpcServers, err := createMoons(moonNum, "127.0.0.1:3260", dbBasePath)
	assert.NoError(t, err)

	for i := 0; i < moonNum; i++ {
		go func(server *messenger.RpcServer) {
			err = server.Run()
			if err != nil {
				t.Errorf("Run rpcServer err: %v", err)
			}
		}(rpcServers[i])
		go moons[i].Run()
	}

	//time.Sleep(5 * time.Second)

	leader := -1
	for {
		ok := true
		for i := 0; i < moonNum; i++ {
			if moons[i].raft == nil {
				ok = false
				break
			}
			if moons[i].raft.Status().Lead == 0 || len(moons[i].InfoStorage.ListAllNodeInfo()) != moonNum {
				ok = false
			}
			leader = int(moons[i].raft.Status().Lead)
		}
		if !ok {
			time.Sleep(100 * time.Millisecond)
			continue
		} else {
			t.Logf("leader: %v", leader)
			break
		}
	}
	for i := 0; i < moonNum; i++ {
		rpcServers[i].Stop()
		moons[i].Stop()
	}
}

func createMoons(num int, sunAddr string, basePath string) ([]*Moon, []*messenger.RpcServer, error) {
	err := common.InitAndClearPath(basePath)
	if err != nil {
		return nil, nil, err
	}
	var infoStorages []node.InfoStorage
	var stableStorages []Storage
	var rpcServers []*messenger.RpcServer
	var moons []*Moon
	var nodeInfos []*node.NodeInfo

	for i := 0; i < num; i++ {
		raftID := uint64(i + 1)
		//infoStorages = append(infoStorages,
		//	node.NewStableNodeInfoStorage(path.Join(basePath, "/nodeInfo", strconv.Itoa(i+1))))
		infoStorages = append(infoStorages, node.NewMemoryNodeInfoStorage())
		stableStorages = append(stableStorages, NewStorage(path.Join(basePath, "/raft", strconv.Itoa(i+1))))
		rpcServers = append(rpcServers, messenger.NewRpcServer(32670+raftID))
		nodeInfos = append(nodeInfos, node.NewSelfInfo(raftID, "127.0.0.1", 32670+raftID))
	}

	moonConfig := DefaultConfig
	moonConfig.SunAddr = sunAddr
	moonConfig.GroupInfo = node.GroupInfo{
		GroupTerm:       &node.Term{Term: 0},
		LeaderInfo:      nil,
		UpdateTimestamp: timestamp.Now(),
	}

	for i := 0; i < num; i++ {
		if sunAddr != "" {
			moons = append(moons, NewMoon(nodeInfos[i], &moonConfig, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		} else {
			moonConfig.GroupInfo.NodesInfo = nodeInfos
			moons = append(moons, NewMoon(nodeInfos[i], &moonConfig, rpcServers[i], infoStorages[i],
				stableStorages[i]))
		}
	}
	return moons, rpcServers, nil
}
