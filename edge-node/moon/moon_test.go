package moon

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	common2 "ecos/messenger/common"
	"ecos/utils/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestRaft(t *testing.T) {
	basePath := "./ecos-data/db/moon"
	nodeNum := 9
	ctx := context.Background()
	moons, rpcServers, err := createMoons(ctx, nodeNum, basePath)
	assert.NoError(t, err)

	// 启动所有 moon 节点
	var nodeInfos []*infos.NodeInfo
	for i := 0; i < nodeNum; i++ {
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
		for i := 0; i < nodeNum; i++ {
			rpcServers[i].Stop()
			moons[i].Stop()
		}
		_ = os.RemoveAll(basePath)
	})

	var leader int
	// 等待选主
	t.Run("get leader", func(t *testing.T) {
		leader = waitMoonsOK(moons)
		t.Logf("leader: %v", leader)
	})
	// 发送一个待同步的 info
	t.Run("propose info", func(t *testing.T) {
		moon := moons[leader-1]
		request := &ProposeInfoRequest{
			Head: &common2.Head{
				Timestamp: timestamp.Now(),
				Term:      0,
			},
			Operate: ProposeInfoRequest_ADD,
			Id:      "666",
			BaseInfo: &infos.BaseInfo{
				Info: &infos.BaseInfo_ClusterInfo{ClusterInfo: &infos.ClusterInfo{
					Term: 666,
					LeaderInfo: &infos.NodeInfo{
						RaftId:   6,
						Uuid:     "66",
						IpAddr:   "",
						RpcPort:  0,
						Capacity: 0,
					},
					NodesInfo:       nil,
					UpdateTimestamp: nil,
				}},
			},
		}
		_, err = moon.ProposeInfo(context.Background(), request)
		assert.NoError(t, err)
		time.Sleep(time.Second * 1)
		storage := moons[nodeNum-1].infoStorageRegister.GetStorage(infos.InfoType_CLUSTER_INFO)
		info, err := storage.Get("666")
		assert.NoError(t, err)
		assert.Equal(t, request.BaseInfo.GetClusterInfo().LeaderInfo.RaftId,
			info.BaseInfo().GetClusterInfo().LeaderInfo.RaftId)

	})
}

func waitMoonsOK(moons []*Moon) int {
	leader := -1
	for {
		ok := true
		for i := 0; i < len(moons); i++ {
			if moons[i].GetLeaderID() == 0 {
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

func createMoons(ctx context.Context, num int, basePath string) ([]*Moon, []*messenger.RpcServer, error) {
	err := common.InitAndClearPath(basePath)
	if err != nil {
		return nil, nil, err
	}
	var rpcServers []*messenger.RpcServer
	var moons []*Moon
	var nodeInfos []*infos.NodeInfo
	var moonConfigs []*Config

	for i := 0; i < num; i++ {
		raftID := uint64(i + 1)
		port, rpcServer := messenger.NewRandomPortRpcServer()
		rpcServers = append(rpcServers, rpcServer)
		nodeInfos = append(nodeInfos, infos.NewSelfInfo(raftID, "127.0.0.1", port))
		moonConfig := DefaultConfig
		moonConfig.ClusterInfo = infos.ClusterInfo{
			Term:            0,
			LeaderInfo:      nil,
			UpdateTimestamp: timestamp.Now(),
		}
		moonConfig.RaftStoragePath = path.Join(basePath, "raft", strconv.Itoa(i+1))
		moonConfigs = append(moonConfigs, &moonConfig)
	}

	for i := 0; i < num; i++ {
		moonConfigs[i].ClusterInfo.NodesInfo = nodeInfos
		builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
		register := builder.GetStorageRegister()
		moons = append(moons, NewMoon(ctx, nodeInfos[i], moonConfigs[i], rpcServers[i],
			register))
	}
	return moons, rpcServers, nil
}
