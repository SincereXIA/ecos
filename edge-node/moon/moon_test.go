package moon

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	common2 "ecos/messenger/common"
	moon2 "ecos/shared/moon"
	"ecos/utils/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestMoon(t *testing.T) {
	t.Run("Real Moon", func(t *testing.T) {
		testMoon(t, false)
	})
	t.Run("Mock Moon", func(t *testing.T) {
		testMoon(t, true)
	})
}

func testMoon(t *testing.T, mock bool) {
	basePath := "./ecos-data/db/moon"
	nodeNum := 9
	ctx := context.Background()
	var moons []InfoController
	var rpcServers []*messenger.RpcServer
	var err error

	if mock {
		isLeader := true
		builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
		register := builder.GetStorageRegister()

		for i := 0; i < nodeNum; i++ {
			raftID := uint64(i + 1)
			ctrl := gomock.NewController(t)
			moon := NewMockInfoController(ctrl)
			port, rpcServer := messenger.NewRandomPortRpcServer()
			nodeInfo := infos.NewSelfInfo(raftID, "127.0.0.1", port)

			InitMock(moon, rpcServer, register, nodeInfo, isLeader)
			isLeader = false
			moons = append(moons, moon)
			rpcServers = append(rpcServers, rpcServer)
		}
	} else {
		moons, rpcServers, err = createMoons(ctx, nodeNum, basePath)
		assert.NoError(t, err)
	}
	// 启动所有 moon 节点
	for i := 0; i < nodeNum; i++ {
		index := i
		go func() {
			err := rpcServers[index].Run()
			if err != nil {
				t.Errorf("Run rpcServer err: %v", err)
			}
		}()
		go moons[i].Run()
	}
	time.Sleep(time.Second) // 保证所有 moons 都执行了 run 方法

	t.Cleanup(func() {
		for i := 0; i < nodeNum; i++ {
			moons[i].Stop()
			rpcServers[i].Stop()
		}
		// _ = os.RemoveAll(basePath)
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
		request := &moon2.ProposeInfoRequest{
			Head: &common2.Head{
				Timestamp: timestamp.Now(),
				Term:      0,
			},
			Operate: moon2.ProposeInfoRequest_ADD,
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
		_, err := moon.ProposeInfo(context.Background(), request)
		assert.NoError(t, err)
	})

	t.Run("propose bucket info", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			moon := moons[leader-1]
			request := &moon2.ProposeInfoRequest{
				Operate: moon2.ProposeInfoRequest_ADD,
				Id:      "/root/bucket" + strconv.Itoa(i),
				BaseInfo: &infos.BaseInfo{
					Info: &infos.BaseInfo_BucketInfo{
						BucketInfo: &infos.BucketInfo{
							VolumeId:   "/root/",
							BucketName: "bucket" + strconv.Itoa(i),
							UserId:     "root",
							GroupId:    "root",
							Mode:       0,
							Config:     nil,
						},
					},
				},
			}
			_, err := moon.ProposeInfo(context.Background(), request)
			assert.NoError(t, err)
		}
	})

	t.Run("get info", func(t *testing.T) {
		moon := moons[leader-1]
		request := &moon2.GetInfoRequest{
			Head: &common2.Head{
				Timestamp: timestamp.Now(),
				Term:      0,
			},
			InfoType: infos.InfoType_CLUSTER_INFO,
			InfoId:   "666",
		}
		response, err := moon.GetInfo(ctx, request)
		assert.NoError(t, err)
		assert.Equal(t, uint64(6), response.BaseInfo.GetClusterInfo().LeaderInfo.RaftId)
	})

}

func waitMoonsOK(moons []InfoController) int {
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

func createMoons(ctx context.Context, num int, basePath string) ([]InfoController, []*messenger.RpcServer, error) {
	err := common.InitPath(basePath)
	if err != nil {
		return nil, nil, err
	}
	var rpcServers []*messenger.RpcServer
	var moons []InfoController
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
		moonConfig.RocksdbStoragePath = path.Join(basePath, "rocksdb", strconv.Itoa(i+1))
		moonConfigs = append(moonConfigs, &moonConfig)
	}

	for i := 0; i < num; i++ {
		moonConfigs[i].ClusterInfo.NodesInfo = nodeInfos
		// builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
		builder := infos.NewStorageRegisterBuilder(infos.NewRocksDBInfoStorageFactory(basePath + strconv.FormatInt(int64(i), 10)))
		register := builder.GetStorageRegister()
		moons = append(moons, NewMoon(ctx, nodeInfos[i], moonConfigs[i], rpcServers[i], register))
	}
	return moons, rpcServers, nil
}
