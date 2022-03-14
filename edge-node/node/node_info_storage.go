package node

import (
	"ecos/messenger/timestamppb"
	"errors"
	"github.com/gogo/protobuf/sortkeys"
	"github.com/mohae/deepcopy"
)

type ID uint64

// InfoStorage 实现有任期的边缘节点信息存储
//
type InfoStorage interface {
	UpdateNodeInfo(info *NodeInfo) error
	DeleteNodeInfo(nodeId ID) error
	GetNodeInfo(nodeId ID) (*NodeInfo, error)
	SetLeader(nodeId ID) error
	// Commit 提交缓存区的节点信息，至此该 term 无法再变化，但提交尚未生效
	Commit(newTerm uint64)
	// Apply 使得已经 Commit 的节点信息马上生效
	Apply()

	// ListAllNodeInfo 用于节点间通信时获取节点信息
	ListAllNodeInfo() map[ID]*NodeInfo
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
}

func (storage *MemoryNodeInfoStorage) UpdateNodeInfo(info *NodeInfo) error {
	nodeId := info.RaftId
	storage.uncommittedState.InfoMap[nodeId] = info
	storage.updateTimestamp()
	return nil
}

func (storage *MemoryNodeInfoStorage) DeleteNodeInfo(nodeId ID) error {
	delete(storage.uncommittedState.InfoMap, uint64(nodeId))
	storage.updateTimestamp()
	return nil
}

func (storage *MemoryNodeInfoStorage) GetNodeInfo(nodeId ID) (*NodeInfo, error) {
	if nodeInfo, ok := storage.uncommittedState.InfoMap[uint64(nodeId)]; ok {
		return nodeInfo, nil
	}
	return nil, errors.New("node not found")

}

func (storage *MemoryNodeInfoStorage) ListAllNodeInfo() map[ID]*NodeInfo {
	tempMap := make(map[ID]*NodeInfo)
	for k, v := range storage.uncommittedState.InfoMap {
		tempMap[ID(k)] = v
	}
	return tempMap
}

func (storage *MemoryNodeInfoStorage) Commit(newTerm uint64) {
	// copy from uncommittedInfoMap
	storage.uncommittedState.Term = newTerm
	cpy := deepcopy.Copy(storage.uncommittedState)
	storage.committedState = cpy.(*InfoStorageState)
}

func (storage *MemoryNodeInfoStorage) Apply() {
	// Apply
	cpy := deepcopy.Copy(storage.committedState)

	storage.history.HistoryMap[storage.nowState.Term] = storage.GetGroupInfo(0)
	storage.nowState = cpy.(*InfoStorageState)
	if storage.onGroupApplyHookFunc != nil {
		storage.onGroupApplyHookFunc(storage.GetGroupInfo(0))
	}
}

func (storage *MemoryNodeInfoStorage) GetGroupInfo(term uint64) *GroupInfo {
	if term == 0 || term == storage.nowState.Term {
		leaderInfo := storage.nowState.InfoMap[storage.nowState.LeaderID]
		return &GroupInfo{
			GroupTerm: &Term{
				Term: storage.nowState.Term,
			},
			LeaderInfo:      leaderInfo,
			NodesInfo:       map2Slice(storage.nowState.InfoMap),
			UpdateTimestamp: storage.nowState.UpdateTimeStamp,
		}
	}
	return storage.history.HistoryMap[term]
}

func (storage *MemoryNodeInfoStorage) SetLeader(nodeId ID) error {
	if _, ok := storage.uncommittedState.InfoMap[uint64(nodeId)]; ok {
		storage.uncommittedState.LeaderID = uint64(nodeId)
		storage.updateTimestamp()
	} else {
		return errors.New("leader not found")
	}
	return nil
}

func (storage *MemoryNodeInfoStorage) SetOnGroupApply(hookFunc func(info *GroupInfo)) {
	storage.onGroupApplyHookFunc = hookFunc
}

func (storage *MemoryNodeInfoStorage) GetTermNow() uint64 {
	return storage.nowState.Term
}

func (storage *MemoryNodeInfoStorage) GetTermList() []uint64 {
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
			UpdateTimeStamp: timestamppb.Now(),
		},
		committedState: &InfoStorageState{
			Term:            0,
			LeaderID:        0,
			InfoMap:         make(map[uint64]*NodeInfo),
			UpdateTimeStamp: timestamppb.Now(),
		},
		uncommittedState: &InfoStorageState{
			Term:            1,
			LeaderID:        0,
			InfoMap:         make(map[uint64]*NodeInfo),
			UpdateTimeStamp: timestamppb.Now(),
		},
		history: History{
			HistoryMap: map[uint64]*GroupInfo{},
		},
	}
}

func map2Slice(input map[uint64]*NodeInfo) (output []*NodeInfo) {
	for _, info := range input {
		output = append(output, info)
	}
	return
}

func (storage *MemoryNodeInfoStorage) updateTimestamp() {
	storage.uncommittedState.UpdateTimeStamp = timestamppb.Now()
}
