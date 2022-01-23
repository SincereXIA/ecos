package moon

type NodeID uint64

type NodeInfoStorage interface {
	UpdateNodeInfo(info *NodeInfo) error
	DeleteNodeInfo(nodeId NodeID) error
	GetNodeInfo(nodeId NodeID) (error, NodeInfo)
	ListAllNodeInfo() map[NodeID]NodeInfo
}

type MemoryNodeInfoStorage struct {
	infoMap map[NodeID]NodeInfo
}

func (storage *MemoryNodeInfoStorage) UpdateNodeInfo(info *NodeInfo) error {
	nodeId := info.ID
	storage.infoMap[nodeId] = *info
	return nil
}

func (storage *MemoryNodeInfoStorage) DeleteNodeInfo(nodeId NodeID) error {
	delete(storage.infoMap, nodeId)
	return nil
}

func (storage *MemoryNodeInfoStorage) GetNodeInfo(nodeId NodeID) (error, NodeInfo) {
	return nil, storage.infoMap[nodeId]
}

func (storage *MemoryNodeInfoStorage) ListAllNodeInfo() map[NodeID]NodeInfo {
	return storage.infoMap
}

func NewMemoryNodeInfoStorage() *MemoryNodeInfoStorage {
	return &MemoryNodeInfoStorage{infoMap: map[NodeID]NodeInfo{}}
}
