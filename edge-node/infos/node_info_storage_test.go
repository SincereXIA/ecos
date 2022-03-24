package infos

import (
	"ecos/utils/common"
	"ecos/utils/timestamp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestNodeInfoStorage(t *testing.T) {
	storage := NewMemoryNodeInfoStorage()
	testStorage(storage, t)

	testDataBaseDir := "./testNodeInfo"
	_ = common.InitAndClearPath(testDataBaseDir)
	stableStorage := NewStableNodeInfoStorage(testDataBaseDir)
	defer stableStorage.Close()
	testStorage(stableStorage, t)
	_ = os.RemoveAll(testDataBaseDir)
}

func testStorage(storage NodeInfoStorage, t *testing.T) {
	hookInfoNum := 0
	t.Run("test init", func(t *testing.T) {
		clusterInfo := storage.GetClusterInfo(0)
		assert.Equal(t, clusterInfo.Term, uint64(0))
		_, err := storage.GetNodeInfo(0)
		assert.NotNil(t, err)
		storage.SetOnGroupApply(func(info *ClusterInfo) {
			hookInfoNum = len(info.NodesInfo)
		})
	})
	t.Run("test add node info", func(t *testing.T) {
		info1 := NodeInfo{
			RaftId:   1,
			Uuid:     uuid.New().String(),
			IpAddr:   "8.8.8.8",
			RpcPort:  8888,
			Capacity: 8888,
		}
		info2 := NodeInfo{
			RaftId:   2,
			Uuid:     uuid.New().String(),
			IpAddr:   "8.8.8.8",
			RpcPort:  8888,
			Capacity: 8888,
		}
		assert.NoError(t, storage.UpdateNodeInfo(&info1, timestamp.Now()))
		assert.NoError(t, storage.UpdateNodeInfo(&info2, timestamp.Now()))
		infos := storage.ListAllNodeInfo()
		assert.Equal(t, len(infos), 2)
		group := storage.GetClusterInfo(0)
		assert.Empty(t, group.NodesInfo)
	})
	t.Run("test update leader info", func(t *testing.T) {
		assert.NoError(t, storage.SetLeader(1, timestamp.Now()))
	})
	t.Run("test info commit", func(t *testing.T) {
		storage.Commit(uint64(time.Now().UnixNano()))
		group := storage.GetClusterInfo(0)
		assert.Empty(t, group.NodesInfo)
	})
	t.Run("test info apply", func(t *testing.T) {
		storage.Apply()
		group := storage.GetClusterInfo(0)
		assert.NotEmpty(t, group.NodesInfo)
		t.Logf("Old term: %v, new term: %v", 0, group.Term)
		assert.NotEqual(t, group.Term, uint64(0))
		term := group.Term
		info3 := NodeInfo{
			RaftId:   3,
			Uuid:     uuid.New().String(),
			IpAddr:   "8.8.8.8",
			RpcPort:  8888,
			Capacity: 8888,
		}
		assert.NoError(t, storage.UpdateNodeInfo(&info3, timestamp.Now()))
		group = storage.GetClusterInfo(0)
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Commit(uint64(time.Now().UnixNano()))
		// TODO:  rocksdb 的 infoStorage 完成后，取消下面注释
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Apply()
		group = storage.GetClusterInfo(0)
		t.Logf("Old term: %v, new term: %v", term, group.Term)
		assert.NotEqual(t, group.Term, term)
		assert.Equal(t, len(group.NodesInfo), 3)
	})
	t.Run("test hook", func(t *testing.T) {
		assert.Equal(t, 3, hookInfoNum)
	})
}