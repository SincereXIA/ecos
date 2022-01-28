package node

import "errors"

type NodeID uint64

type NodeInfoStorage interface {
	UpdateNodeInfo(info *NodeInfo) error
	DeleteNodeInfo(nodeId NodeID) error
	GetNodeInfo(nodeId NodeID) (*NodeInfo, error)
	ListAllNodeInfo() map[NodeID]NodeInfo
}

type MemoryNodeInfoStorage struct {
	infoMap map[NodeID]NodeInfo
}

func (storage *MemoryNodeInfoStorage) UpdateNodeInfo(info *NodeInfo) error {
	nodeId := info.RaftId
	storage.infoMap[NodeID(nodeId)] = *info
	return nil
}

func (storage *MemoryNodeInfoStorage) DeleteNodeInfo(nodeId NodeID) error {
	delete(storage.infoMap, nodeId)
	return nil
}

func (storage *MemoryNodeInfoStorage) GetNodeInfo(nodeId NodeID) (*NodeInfo, error) {
	if nodeInfo, ok := storage.infoMap[nodeId]; ok {
		return &nodeInfo, nil
	}
	return nil, errors.New("node not found")

}

func (storage *MemoryNodeInfoStorage) ListAllNodeInfo() map[NodeID]NodeInfo {
	return storage.infoMap
}

func NewMemoryNodeInfoStorage() *MemoryNodeInfoStorage {
	return &MemoryNodeInfoStorage{infoMap: map[NodeID]NodeInfo{}}
}
