package node

import (
	"ecos/utils/common"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestMemoryNodeInfoStorage(t *testing.T) {
	storage := NewMemoryNodeInfoStorage()
	testStorage(storage, t)

	testDataBaseDir := "./testNodeInfo"
	_ = common.InitAndClearPath(testDataBaseDir)
	stableStorage := NewStableNodeInfoStorage(testDataBaseDir)
	defer stableStorage.Close()
	testStableStorage(stableStorage, t)
	_ = os.RemoveAll(testDataBaseDir)
}

func testStorage(storage InfoStorage, t *testing.T) {
	hookInfoNum := 0
	t.Run("test init", func(t *testing.T) {
		groupInfo := storage.GetGroupInfo(0)
		assert.Equal(t, groupInfo.GroupTerm.Term, uint64(0))
		_, err := storage.GetNodeInfo(0)
		assert.NotNil(t, err)
		storage.SetOnGroupApply(func(info *GroupInfo) {
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
		assert.NoError(t, storage.UpdateNodeInfo(&info1))
		assert.NoError(t, storage.UpdateNodeInfo(&info2))
		infos := storage.ListAllNodeInfo()
		assert.Equal(t, len(infos), 2)
		group := storage.GetGroupInfo(0)
		assert.Empty(t, group.NodesInfo)
	})
	t.Run("test update leader info", func(t *testing.T) {
		assert.NoError(t, storage.SetLeader(1))
	})
	t.Run("test info commit", func(t *testing.T) {
		storage.Commit(uint64(time.Now().UnixNano()))
		// TODO:  rocksdb 的 infoStorage 完成后，取消下面注释
		//group := storage.GetGroupInfo()
		//assert.Empty(t, group.NodesInfo)
	})
	t.Run("test info apply", func(t *testing.T) {
		storage.Apply()
		group := storage.GetGroupInfo(0)
		assert.NotEmpty(t, group.NodesInfo)
		t.Logf("Old term: %v, new term: %v", 0, group.GroupTerm.Term)
		assert.NotEqual(t, group.GroupTerm.Term, uint64(0))
		term := group.GroupTerm.Term
		info3 := NodeInfo{
			RaftId:   3,
			Uuid:     uuid.New().String(),
			IpAddr:   "8.8.8.8",
			RpcPort:  8888,
			Capacity: 8888,
		}
		assert.NoError(t, storage.UpdateNodeInfo(&info3))
		group = storage.GetGroupInfo(0)
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Commit(uint64(time.Now().UnixNano()))
		// TODO:  rocksdb 的 infoStorage 完成后，取消下面注释
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Apply()
		group = storage.GetGroupInfo(0)
		t.Logf("Old term: %v, new term: %v", term, group.GroupTerm.Term)
		assert.NotEqual(t, group.GroupTerm.Term, term)
		assert.Equal(t, len(group.NodesInfo), 3)
	})
	t.Run("test hook", func(t *testing.T) {
		assert.Equal(t, 3, hookInfoNum)
	})
}

func testStableStorage(storage *StableNodeInfoStorage, t *testing.T) {
	hookInfoNum := 0
	t.Run("test init", func(t *testing.T) {
		groupInfo := storage.GetGroupInfo(0)
		assert.Equal(t, groupInfo.GroupTerm.Term, uint64(0))
		_, err := storage.GetNodeInfo(0)
		assert.NotNil(t, err)
		storage.SetOnGroupApply(func(info *GroupInfo) {
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
		assert.NoError(t, storage.UpdateNodeInfo(&info1))
		assert.NoError(t, storage.UpdateNodeInfo(&info2))
		infos := storage.ListAllNodeInfo()
		assert.Equal(t, len(infos), 2)
		group := storage.GetGroupInfo(0)
		assert.Empty(t, group.NodesInfo)
	})
	t.Run("test update leader info", func(t *testing.T) {
		assert.NoError(t, storage.SetLeader(1))
	})
	t.Run("test info commit", func(t *testing.T) {
		storage.Commit(uint64(time.Now().UnixNano()))
		// TODO:  rocksdb 的 infoStorage 完成后，取消下面注释
		//group := storage.GetGroupInfo()
		//assert.Empty(t, group.NodesInfo)
	})
	t.Run("test info apply", func(t *testing.T) {
		storage.Apply()
		group := storage.GetGroupInfo(0)
		assert.NotEmpty(t, group.NodesInfo)
		t.Logf("Old term: %v, new term: %v", 0, group.GroupTerm.Term)
		assert.NotEqual(t, group.GroupTerm.Term, uint64(0))
		term := group.GroupTerm.Term
		info3 := NodeInfo{
			RaftId:   3,
			Uuid:     uuid.New().String(),
			IpAddr:   "8.8.8.8",
			RpcPort:  8888,
			Capacity: 8888,
		}
		assert.NoError(t, storage.UpdateNodeInfo(&info3))
		group = storage.GetGroupInfo(0)
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Commit(uint64(time.Now().UnixNano()))
		// TODO:  rocksdb 的 infoStorage 完成后，取消下面注释
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Apply()
		group = storage.GetGroupInfo(0)
		t.Logf("Old term: %v, new term: %v", term, group.GroupTerm.Term)
		assert.NotEqual(t, group.GroupTerm.Term, term)
		assert.Equal(t, len(group.NodesInfo), 3)
	})
	t.Run("test hook", func(t *testing.T) {
		assert.Equal(t, 3, hookInfoNum)
	})
	t.Run("test save memory info", func(t *testing.T) {
		storage.SaveMemoryNodeInfoStorage()
	})
}
