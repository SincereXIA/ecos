package node

import (
	"ecos/edge-node/node"
)

// ClientNodeInfoStorage provides way to query NodeInfo and GroupInfo for a specific Term
type ClientNodeInfoStorage struct {
	curGroupInfo *node.GroupInfo
	curNodesInfo map[uint64]*node.NodeInfo
	history      map[uint64]*node.GroupInfo
}

// NewClientNodeInfoStorage generates server-independent NodeInfoStorage
func NewClientNodeInfoStorage() (*ClientNodeInfoStorage, error) {
	return &ClientNodeInfoStorage{
		history: map[uint64]*node.GroupInfo{},
	}, nil
}

// InfoStorage is the ONLY in-memory LOCAL storage for ClientNodeInfoStorage
var InfoStorage *ClientNodeInfoStorage

func init() {
	if InfoStorage == nil {
		InfoStorage, _ = NewClientNodeInfoStorage()
	}
}

// SaveGroupInfo save GroupInfo into history
func (s *ClientNodeInfoStorage) SaveGroupInfo(groupInfo *node.GroupInfo) {
	s.history[groupInfo.GroupTerm.Term] = groupInfo
}

// SaveGroupInfoWithTerm same as SaveGroupInfo, shall check para term and groupInfo.Term
func (s *ClientNodeInfoStorage) SaveGroupInfoWithTerm(term uint64, groupInfo *node.GroupInfo) {
	if term == 0 {
		s.curGroupInfo = groupInfo
		s.curNodesInfo = make(map[uint64]*node.NodeInfo)
		for _, info := range s.curGroupInfo.NodesInfo {
			s.curNodesInfo[info.RaftId] = info
		}
	}
	if term == groupInfo.GroupTerm.Term {
		s.history[term] = groupInfo
	}
}

// GetGroupInfo shall return GroupInfo with given term.
//
// If term is 0, this shall return current GroupInfo
//
// CAN return NIL
func (s *ClientNodeInfoStorage) GetGroupInfo(term uint64) *node.GroupInfo {
	if term == 0 {
		return s.curGroupInfo
	}
	return s.history[term]
}

// GetNodeInfo shall return NodeInfo with given term and nodeId.
//
// If term is 0, this shall search in the current GroupInfo
//
// CAN return NIL
func (s *ClientNodeInfoStorage) GetNodeInfo(term uint64, nodeId uint64) *node.NodeInfo {
	if term == 0 {
		return s.curNodesInfo[nodeId]
	}
	groupInfo := s.GetGroupInfo(term)
	for _, info := range groupInfo.NodesInfo {
		if info.RaftId == nodeId {
			return info
		}
	}
	return nil
}
