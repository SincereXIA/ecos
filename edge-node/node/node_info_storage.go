package node

import (
	"context"
	"errors"
	"github.com/mohae/deepcopy"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	Commit(nextTerm uint64)
	// Apply 使得已经 Commit 的节点信息马上生效
	Apply()

	// ListAllNodeInfo 用于节点间通信时获取节点信息
	ListAllNodeInfo() map[ID]NodeInfo
	// GetGroupInfo 用于为 Crush 算法提供集群信息
	// 默认
	GetGroupInfo() *GroupInfo

	SetOnGroupApply(hookFunc func(info *GroupInfo))

	Close()
}

type MemoryNodeInfoStorage struct {
	nowState         *InfoStorageState
	committedState   *InfoStorageState
	uncommittedState *InfoStorageState

	ctx context.Context

	onGroupApplyHookFunc func(info *GroupInfo)
}

type InfoStorageState struct {
	Term            uint64
	LeaderID        ID
	InfoMap         map[ID]NodeInfo
	UpdateTimeStamp *Timestamp
}

func (storage *MemoryNodeInfoStorage) UpdateNodeInfo(info *NodeInfo) error {
	nodeId := info.RaftId
	storage.uncommittedState.InfoMap[ID(nodeId)] = *info
	storage.updateTimestamp()
	return nil
}

func (storage *MemoryNodeInfoStorage) DeleteNodeInfo(nodeId ID) error {
	delete(storage.uncommittedState.InfoMap, nodeId)
	storage.updateTimestamp()
	return nil
}

func (storage *MemoryNodeInfoStorage) GetNodeInfo(nodeId ID) (*NodeInfo, error) {
	if nodeInfo, ok := storage.uncommittedState.InfoMap[nodeId]; ok {
		return &nodeInfo, nil
	}
	return nil, errors.New("node not found")

}

func (storage *MemoryNodeInfoStorage) ListAllNodeInfo() map[ID]NodeInfo {
	return storage.uncommittedState.InfoMap
}

func (storage *MemoryNodeInfoStorage) Commit(nextTerm uint64) {
	// copy from uncommittedInfoMap
	cpy := deepcopy.Copy(storage.uncommittedState)
	storage.committedState = cpy.(*InfoStorageState)
	// set nextTerm
	storage.uncommittedState.Term = nextTerm
}

func (storage *MemoryNodeInfoStorage) Apply() {
	// Apply
	cpy := deepcopy.Copy(storage.committedState)
	storage.nowState = cpy.(*InfoStorageState)
	if storage.onGroupApplyHookFunc != nil {
		storage.onGroupApplyHookFunc(storage.GetGroupInfo())
	}
}

func (storage *MemoryNodeInfoStorage) GetGroupInfo() *GroupInfo {
	leaderInfo := storage.nowState.InfoMap[storage.nowState.LeaderID]
	return &GroupInfo{
		Term:            storage.nowState.Term,
		LeaderInfo:      &leaderInfo,
		NodesInfo:       map2Slice(storage.nowState.InfoMap),
		UpdateTimestamp: storage.nowState.UpdateTimeStamp,
	}
}

func (storage *MemoryNodeInfoStorage) SetLeader(nodeId ID) error {
	if _, ok := storage.uncommittedState.InfoMap[nodeId]; ok {
		storage.uncommittedState.LeaderID = nodeId
		storage.updateTimestamp()
	} else {
		return errors.New("leader not found")
	}
	return nil
}

func (storage *MemoryNodeInfoStorage) SetOnGroupApply(hookFunc func(info *GroupInfo)) {
	storage.onGroupApplyHookFunc = hookFunc
}

func (storage *MemoryNodeInfoStorage) Close() {
	// do nothing
}

func NewMemoryNodeInfoStorage() *MemoryNodeInfoStorage {
	return &MemoryNodeInfoStorage{
		nowState: &InfoStorageState{
			Term:            0,
			LeaderID:        0,
			InfoMap:         make(map[ID]NodeInfo),
			UpdateTimeStamp: timestamppb.Now(),
		},
		committedState: &InfoStorageState{
			Term:            0,
			LeaderID:        0,
			InfoMap:         make(map[ID]NodeInfo),
			UpdateTimeStamp: timestamppb.Now(),
		},
		uncommittedState: &InfoStorageState{
			Term:            1,
			LeaderID:        0,
			InfoMap:         make(map[ID]NodeInfo),
			UpdateTimeStamp: timestamppb.Now(),
		},
	}
}

func map2Slice(input map[ID]NodeInfo) (output []*NodeInfo) {
	for _, value := range input {
		output = append(output, &value)
	}
	return
}

func (storage *MemoryNodeInfoStorage) updateTimestamp() {
	storage.uncommittedState.UpdateTimeStamp = timestamppb.Now()
}
