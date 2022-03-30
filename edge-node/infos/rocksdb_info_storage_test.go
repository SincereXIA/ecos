package infos

import (
	"ecos/utils/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/testing/protocmp"
	"os"
	"testing"
)

func TestRocksDBInfoStorage(t *testing.T) {
	testDataBaseDir := "./testStorageInfo"
	storageFactory := NewRocksDBInfoStorageFactory(testDataBaseDir)
	nodeInfoStorage := storageFactory.GetStorage(InfoType_NODE_INFO)
	testStorage(nodeInfoStorage, t)
	testStorageAndGet(nodeInfoStorage, t)
	nodeInfoStorage.Close()
	clusterIndoStorege := storageFactory.GetStorage(InfoType_CLUSTER_INFO)
	testStorage(clusterIndoStorege, t)
	clusterIndoStorege.Close()
	_ = os.RemoveAll(testDataBaseDir)
}

func testStorage(storage Storage, t *testing.T) {
	t.Run("test add node_info and cluster_info", func(t *testing.T) {
		nodeInfo1 := &BaseInfo_NodeInfo{
			NodeInfo: &NodeInfo{
				RaftId:   1,
				Uuid:     uuid.New().String(),
				IpAddr:   "8.8.8.8",
				RpcPort:  8888,
				Capacity: 8888,
			},
		}
		nodeInfo2 := &BaseInfo_NodeInfo{
			NodeInfo: &NodeInfo{
				RaftId:   2,
				Uuid:     uuid.New().String(),
				IpAddr:   "8.8.8.8",
				RpcPort:  8888,
				Capacity: 8888,
			},
		}

		baseNodeInfo1 := &BaseInfo{
			Info: isBaseInfo_Info(nodeInfo1),
		}
		baseNodeInfo2 := &BaseInfo{
			Info: isBaseInfo_Info(nodeInfo2),
		}
		assert.NoError(t, storage.Update(baseNodeInfo1))
		assert.NoError(t, storage.Update(baseNodeInfo2))
		infos, err := storage.GetAll()
		assert.NoError(t, err)
		assert.Equal(t, len(infos), 2)

		nodeInfos := []*NodeInfo{nodeInfo1.NodeInfo, nodeInfo2.NodeInfo}

		clusterInfo1 := &BaseInfo_ClusterInfo{
			ClusterInfo: &ClusterInfo{
				Term:            1,
				LeaderInfo:      nodeInfo1.NodeInfo,
				NodesInfo:       nodeInfos,
				UpdateTimestamp: timestamp.Now(),
			},
		}

		clusterInfo2 := &BaseInfo_ClusterInfo{
			ClusterInfo: &ClusterInfo{
				Term:            2,
				LeaderInfo:      nodeInfo1.NodeInfo,
				NodesInfo:       nodeInfos,
				UpdateTimestamp: timestamp.Now(),
			},
		}

		baseClusterInfo1 := &BaseInfo{
			Info: isBaseInfo_Info(clusterInfo1),
		}
		baseClusterInfo2 := &BaseInfo{
			Info: isBaseInfo_Info(clusterInfo2),
		}
		assert.NoError(t, storage.Update(baseClusterInfo1))
		assert.NoError(t, storage.Update(baseClusterInfo2))
	})

}

func testStorageAndGet(storage Storage, t *testing.T) {
	nodeInfo3 := &BaseInfo_NodeInfo{
		NodeInfo: &NodeInfo{
			RaftId:   3,
			Uuid:     uuid.New().String(),
			IpAddr:   "8.8.8.8",
			RpcPort:  8888,
			Capacity: 8888,
		},
	}
	baseNodeInfo3 := &BaseInfo{
		Info: isBaseInfo_Info(nodeInfo3),
	}
	storage.Update(baseNodeInfo3)
	info, err := storage.Get(baseNodeInfo3.GetNodeInfo().GetID())
	flag := true
	if diff := cmp.Diff(info, baseNodeInfo3, protocmp.Transform()); diff != "" {
		flag = false
	}
	assert.Equal(t, true, flag)
	assert.NoError(t, err)
}
