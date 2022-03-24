package infos

import (
	"ecos/utils/timestamp"
	"errors"
	"github.com/gogo/protobuf/sortkeys"
	"github.com/mohae/deepcopy"
	"sort"
	"sync"
)

type NodeID uint64

// NodeInfoStorage 实现有任期的边缘节点信息存储
//
type NodeInfoStorage interface {
	UpdateNodeInfo(info *NodeInfo, time *timestamp.Timestamp) error
	DeleteNodeInfo(nodeId NodeID, time *timestamp.Timestamp) error
	GetNodeInfo(nodeId NodeID) (*NodeInfo, error)
	SetLeader(nodeId NodeID, time *timestamp.Timestamp) error
	// Commit 提交缓存区的节点信息，至此该 term 无法再变化，但提交尚未生效
	Commit(newTerm uint64)
	// Apply 使得已经 Commit 的节点信息马上生效
	Apply()

	// ListAllNodeInfo 用于节点间通信时获取节点信息
	ListAllNodeInfo() map[NodeID]*NodeInfo
	// GetGroupInfo 用于为 Crush 算法提供集群信息
	// 默认
	GetGroupInfo(term uint64) *GroupInfo

	SetOnGroupApply(hookFunc func(info *GroupInfo))

	GetTermNow() uint64
	GetTermList() []uint64
	Close()
}

type MemoryNodeInfoStorage struct {
	history History

	nowState         *InfoStorageState
	committedState   *InfoStorageState
	uncommittedState *InfoStorageState

	onGroupApplyHookFunc func(info *GroupInfo)

	rwMutex sync.RWMutex
}

func (storage *MemoryNodeInfoStorage) UpdateNodeInfo(info *NodeInfo, time *timestamp.Timestamp) error {
	nodeId := info.RaftId
	storage.rwMutex.Lock()
	storage.uncommittedState.InfoMap[nodeId] = info
	storage.updateTimestamp(time)
	storage.rwMutex.Unlock()
	return nil
}

func (storage *MemoryNodeInfoStorage) DeleteNodeInfo(nodeId NodeID, time *timestamp.Timestamp) error {
	storage.rwMutex.Lock()
	delete(storage.uncommittedState.InfoMap, uint64(nodeId))
	storage.updateTimestamp(time)
	storage.rwMutex.Unlock()
	return nil
}

func (storage *MemoryNodeInfoStorage) GetNodeInfo(nodeId NodeID) (*NodeInfo, error) {
	storage.rwMutex.RLock()
	defer storage.rwMutex.RUnlock()
	if nodeInfo, ok := storage.uncommittedState.InfoMap[uint64(nodeId)]; ok {
		return nodeInfo, nil
	}
	return nil, errors.New("node not found")
}

func (storage *MemoryNodeInfoStorage) ListAllNodeInfo() map[NodeID]*NodeInfo {
	tempMap := make(map[NodeID]*NodeInfo)
	storage.rwMutex.RLock()
	defer storage.rwMutex.RUnlock()
	for k, v := range storage.uncommittedState.InfoMap {
		tempMap[NodeID(k)] = v
	}
	return tempMap
}

func (storage *MemoryNodeInfoStorage) Commit(newTerm uint64) {
	// copy from uncommittedInfoMap
	storage.rwMutex.Lock()
	defer storage.rwMutex.Unlock()
	storage.uncommittedState.Term = newTerm
	cpy := deepcopy.Copy(storage.uncommittedState)
	storage.committedState = cpy.(*InfoStorageState)
}

func (storage *MemoryNodeInfoStorage) Apply() {
	// Apply
	storage.rwMutex.RLock()
	cpy := deepcopy.Copy(storage.committedState)
	storage.rwMutex.RUnlock()

	nowGroupInfo := storage.GetGroupInfo(0)

	storage.rwMutex.Lock()
	storage.history.HistoryMap[storage.nowState.Term] = nowGroupInfo
	storage.nowState = cpy.(*InfoStorageState)
	storage.rwMutex.Unlock()
	if storage.onGroupApplyHookFunc != nil {
		storage.onGroupApplyHookFunc(storage.GetGroupInfo(0))
	}
}

func (storage *MemoryNodeInfoStorage) GetGroupInfo(term uint64) *GroupInfo {
	storage.rwMutex.RLock()
	defer storage.rwMutex.RUnlock()
	if term == 0 || term == storage.nowState.Term {
		leaderInfo := storage.nowState.InfoMap[storage.nowState.LeaderID]
		return &GroupInfo{
			Term:            storage.nowState.Term,
			LeaderInfo:      leaderInfo,
			NodesInfo:       map2Slice(storage.nowState.InfoMap),
			UpdateTimestamp: storage.nowState.UpdateTimeStamp,
		}
	}
	return storage.history.HistoryMap[term]
}

func (storage *MemoryNodeInfoStorage) SetLeader(nodeId NodeID, time *timestamp.Timestamp) error {
	storage.rwMutex.Lock()
	defer storage.rwMutex.Unlock()
	if _, ok := storage.uncommittedState.InfoMap[uint64(nodeId)]; ok {
		storage.uncommittedState.LeaderID = uint64(nodeId)
		storage.updateTimestamp(time)
	} else {
		return errors.New("leader not found")
	}
	return nil
}

func (storage *MemoryNodeInfoStorage) SetOnGroupApply(hookFunc func(info *GroupInfo)) {
	storage.rwMutex.Lock()
	defer storage.rwMutex.Unlock()
	storage.onGroupApplyHookFunc = hookFunc
}

func (storage *MemoryNodeInfoStorage) GetTermNow() uint64 {
	storage.rwMutex.RLock()
	defer storage.rwMutex.RUnlock()
	return storage.nowState.Term
}

func (storage *MemoryNodeInfoStorage) GetTermList() []uint64 {
	storage.rwMutex.RLock()
	defer storage.rwMutex.RUnlock()
	keys := make([]uint64, 0, len(storage.history.HistoryMap))
	for key := range storage.history.HistoryMap {
		keys = append(keys, key)
	}
	sortkeys.Uint64s(keys)
	return keys
}

func (storage *MemoryNodeInfoStorage) Close() {
	// do nothing
}

func NewMemoryNodeInfoStorage() *MemoryNodeInfoStorage {
	return &MemoryNodeInfoStorage{
		nowState: &InfoStorageState{
			Term:            0,
			LeaderID:        0,
			InfoMap:         make(map[uint64]*NodeInfo),
			UpdateTimeStamp: timestamp.Now(),
		},
		committedState: &InfoStorageState{
			Term:            0,
			LeaderID:        0,
			InfoMap:         make(map[uint64]*NodeInfo),
			UpdateTimeStamp: timestamp.Now(),
		},
		uncommittedState: &InfoStorageState{
			Term:            1,
			LeaderID:        0,
			InfoMap:         make(map[uint64]*NodeInfo),
			UpdateTimeStamp: timestamp.Now(),
		},
		history: History{
			HistoryMap: map[uint64]*GroupInfo{},
		},
		rwMutex: sync.RWMutex{},
	}
}

func map2Slice(input map[uint64]*NodeInfo) (output []*NodeInfo) {
	for _, info := range input {
		output = append(output, info)
	}
	sort.Slice(output, func(i, j int) bool {
		return output[i].RaftId > output[j].RaftId
	})
	return
}

func (storage *MemoryNodeInfoStorage) updateTimestamp(timestamp *timestamp.Timestamp) {
	storage.uncommittedState.UpdateTimeStamp = timestamp
}
