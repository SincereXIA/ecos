package node

import (
	"ecos/edge-node/infos"
)

// ClientNodeInfoStorage provides way to query NodeInfo and ClusterInfo for a specific Term
type ClientNodeInfoStorage struct {
	curClusterInfo *infos.ClusterInfo
	curNodesInfo   map[uint64]*infos.NodeInfo
	history        map[uint64]*infos.ClusterInfo
}

// NewClientNodeInfoStorage generates server-independent NodeInfoStorage
func NewClientNodeInfoStorage() (*ClientNodeInfoStorage, error) {
	return &ClientNodeInfoStorage{
		history: map[uint64]*infos.ClusterInfo{},
	}, nil
}

// InfoStorage is the ONLY in-memory LOCAL storage for ClientNodeInfoStorage
var InfoStorage *ClientNodeInfoStorage

func init() {
	if InfoStorage == nil {
		InfoStorage, _ = NewClientNodeInfoStorage()
	}
}

// SaveClusterInfo save ClusterInfo into history
func (s *ClientNodeInfoStorage) SaveClusterInfo(clusterInfo *infos.ClusterInfo) {
	s.history[clusterInfo.Term] = clusterInfo
}

// SaveClusterInfoWithTerm same as SaveClusterInfo, shall check para term and clusterInfo.Term
func (s *ClientNodeInfoStorage) SaveClusterInfoWithTerm(term uint64, clusterInfo *infos.ClusterInfo) {
	if term == 0 {
		s.curClusterInfo = clusterInfo
		s.curNodesInfo = make(map[uint64]*infos.NodeInfo)
		for _, info := range s.curClusterInfo.NodesInfo {
			s.curNodesInfo[info.RaftId] = info
		}
	}
	if term == clusterInfo.Term {
		s.history[term] = clusterInfo
	}
}

// GetClusterInfo shall return ClusterInfo with given term.
//
// If term is 0, this shall return current ClusterInfo
//
// CAN return NIL
func (s *ClientNodeInfoStorage) GetClusterInfo(term uint64) *infos.ClusterInfo {
	if term == 0 {
		return s.curClusterInfo
	}
	return s.history[term]
}

// GetNodeInfo shall return NodeInfo with given term and nodeId.
//
// If term is 0, this shall search in the current ClusterInfo
//
// CAN return NIL
func (s *ClientNodeInfoStorage) GetNodeInfo(term uint64, nodeId uint64) *infos.NodeInfo {
	if term == 0 {
		return s.curNodesInfo[nodeId]
	}
	clusterInfo := s.GetClusterInfo(term)
	for _, info := range clusterInfo.NodesInfo {
		if info.RaftId == nodeId {
			return info
		}
	}
	return nil
}
