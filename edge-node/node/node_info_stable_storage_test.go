package node

import (
	"github.com/google/uuid"
	"go.etcd.io/etcd/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	"testing"
)

func TestStableNodeInfoStorage(t *testing.T) {
	storage := NewStableNodeInfoStorage()
	defer storage.db.Close()
	t.Run("test init", func(t *testing.T) {
		groupInfo := storage.GetGroupInfo()
		assert.Equal(t, groupInfo.Term, uint64(0))
		_, err := storage.GetNodeInfo(0)
		assert.NotNil(t, err)
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
		group := storage.GetGroupInfo()
		assert.Empty(t, group.NodesInfo)
	})
	t.Run("test update leader info", func(t *testing.T) {
		assert.NoError(t, storage.SetLeader(1))
	})
	t.Run("test info commit", func(t *testing.T) {
		storage.Commit()
		group := storage.GetGroupInfo()
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
		assert.NoError(t, storage.UpdateNodeInfo(&info3))
		group = storage.GetGroupInfo()
		assert.Equal(t, len(group.NodesInfo), 2)
		storage.Commit()
		group = storage.GetGroupInfo()
		t.Logf("Old term: %v, new term: %v", term, group.Term)
		assert.NotEqual(t, group.Term, term)
		assert.Equal(t, len(group.NodesInfo), 3)
	})
}