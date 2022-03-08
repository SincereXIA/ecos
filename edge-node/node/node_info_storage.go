package node

import (
	"errors"
	"github.com/mohae/deepcopy"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type ID uint64

type InfoStorage interface {
	UpdateNodeInfo(info *NodeInfo) error
	DeleteNodeInfo(nodeId ID) error
	GetNodeInfo(nodeId ID) (*NodeInfo, error)
	SetLeader(nodeId ID) error
	// Commit 正式启用缓存区的节点信息
	Commit()

	// ListAllNodeInfo 用于节点间通信时获取节点信息
	ListAllNodeInfo() map[ID]NodeInfo
	// GetGroupInfo 用于为 Crush 算法提供集群信息
	GetGroupInfo() *GroupInfo

	Close()
}

type MemoryNodeInfoStorage struct {
	uncommittedInfoMap   map[ID]NodeInfo
	nowInfoMap           map[ID]NodeInfo
	nowGroupInfo         *GroupInfo
	uncommittedGroupInfo *GroupInfo
}

func (storage *MemoryNodeInfoStorage) UpdateNodeInfo(info *NodeInfo) error {
	nodeId := info.RaftId
	storage.uncommittedInfoMap[ID(nodeId)] = *info
	storage.updateTimestamp()
	return nil
}

func (storage *MemoryNodeInfoStorage) DeleteNodeInfo(nodeId ID) error {
	delete(storage.uncommittedInfoMap, nodeId)
	storage.updateTimestamp()
	return nil
}

func (storage *MemoryNodeInfoStorage) GetNodeInfo(nodeId ID) (*NodeInfo, error) {
	if nodeInfo, ok := storage.uncommittedInfoMap[nodeId]; ok {
		return &nodeInfo, nil
	}
	return nil, errors.New("node not found")

}

func (storage *MemoryNodeInfoStorage) ListAllNodeInfo() map[ID]NodeInfo {
	return storage.uncommittedInfoMap
}

func (storage *MemoryNodeInfoStorage) Commit() {
	storage.nowInfoMap = storage.uncommittedInfoMap
	cpy := deepcopy.Copy(storage.uncommittedGroupInfo)
	storage.nowGroupInfo = cpy.(*GroupInfo)
	storage.nowGroupInfo.NodesInfo = map2Slice(storage.nowInfoMap)
	oldTerm := storage.uncommittedGroupInfo.GroupTerm.Term
	storage.uncommittedGroupInfo.GroupTerm.Term = uint64(time.Now().UnixNano())
	// prevent commit too quick let term equal
	if storage.uncommittedGroupInfo.GroupTerm.Term == oldTerm {
		storage.uncommittedGroupInfo.GroupTerm.Term += 1
	}
}

func (storage *MemoryNodeInfoStorage) GetGroupInfo() *GroupInfo {
	return storage.nowGroupInfo
}

func (storage *MemoryNodeInfoStorage) SetLeader(nodeId ID) error {
	if info, ok := storage.uncommittedInfoMap[nodeId]; ok {
		storage.uncommittedGroupInfo.LeaderInfo = &info
		storage.updateTimestamp()
	} else {
		return errors.New("leader not found")
	}
	return nil
}

func (storage *MemoryNodeInfoStorage) Close() {
	// do nothing
}

func NewMemoryNodeInfoStorage() *MemoryNodeInfoStorage {
	return &MemoryNodeInfoStorage{
		uncommittedInfoMap: make(map[ID]NodeInfo),
		nowInfoMap:         make(map[ID]NodeInfo),
		nowGroupInfo: &GroupInfo{
			GroupTerm: &Term{
				Term: 0,
			},
			LeaderInfo:      nil,
			NodesInfo:       nil,
			UpdateTimestamp: timestamppb.Now(),
		},
		uncommittedGroupInfo: &GroupInfo{
			GroupTerm: &Term{
				Term: uint64(time.Now().UnixNano()),
			},
			LeaderInfo:      nil,
			NodesInfo:       nil,
			UpdateTimestamp: timestamppb.Now(),
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
	storage.uncommittedGroupInfo.UpdateTimestamp = timestamppb.Now()
}
