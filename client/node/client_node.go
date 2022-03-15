package node

import (
	"context"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/messenger"
)

// LocalClientNodeInfoStorage provides way to query NodeInfo and GroupInfo for a specific Term
type LocalClientNodeInfoStorage struct {
	curGroupInfo *node.GroupInfo
	curNodesInfo map[uint64]*node.NodeInfo
	history      map[uint64]*node.GroupInfo
}

// ClientNodeInfoStorage provides way to query NodeInfo and GroupInfo for a specific Term
//
// Can GetGroupInfo with remote node
type ClientNodeInfoStorage struct {
	serverNode   *node.NodeInfo
	curGroupInfo *node.GroupInfo
	curNodesInfo map[uint64]*node.NodeInfo
	history      map[uint64]*node.GroupInfo
}

// NewClientNodeInfoStorage generates server-dependent NodeInfoStorage
func NewClientNodeInfoStorage(serverNode *node.NodeInfo) (*ClientNodeInfoStorage, error) {
	ret := &ClientNodeInfoStorage{
		serverNode: serverNode,
		history:    map[uint64]*node.GroupInfo{},
	}
	conn, err := messenger.GetRpcConnByInfo(serverNode)
	if err != nil {
		return nil, nil
	}
	moonClient := moon.NewMoonClient(conn)
	groupInfo, err := moonClient.GetGroupInfo(context.Background(), &node.Term{Term: 0})
	if err != nil {
		return nil, nil
	}
	ret.curGroupInfo = groupInfo
	ret.history[groupInfo.GroupTerm.Term] = groupInfo
	return ret, nil
}

// NewLocalClientNodeInfoStorage generates server-independent NodeInfoStorage
func NewLocalClientNodeInfoStorage() (*LocalClientNodeInfoStorage, error) {
	return &LocalClientNodeInfoStorage{
		history: map[uint64]*node.GroupInfo{},
	}, nil
}

// LocalInfoStorage is the ONLY in-memory LOCAL storage for LocalClientNodeInfoStorage
var LocalInfoStorage *LocalClientNodeInfoStorage

func init() {
	if LocalInfoStorage == nil {
		LocalInfoStorage, _ = NewLocalClientNodeInfoStorage()
	}
}

// SaveGroupInfo save GroupInfo into history
func (s *LocalClientNodeInfoStorage) SaveGroupInfo(groupInfo *node.GroupInfo) {
	s.history[groupInfo.GroupTerm.Term] = groupInfo
}

// SaveGroupInfoWithTerm same as SaveGroupInfo, shall check para term and groupInfo.Term
func (s *LocalClientNodeInfoStorage) SaveGroupInfoWithTerm(term uint64, groupInfo *node.GroupInfo) {
	if term == 0 {
		s.curGroupInfo = groupInfo
		s.curNodesInfo = make(map[uint64]*node.NodeInfo)
		for _, info := range s.curGroupInfo.NodesInfo {
			s.curNodesInfo[info.RaftId] = info
		}
	}
	if term == groupInfo.GroupTerm.Term {
		s.history[term] = groupInfo
	} else {
		// DO NOTHING
	}
}

// GetGroupInfo shall return GroupInfo with given term.
//
// If term is 0, this shall return current GroupInfo
//
// CAN return NIL
func (s *LocalClientNodeInfoStorage) GetGroupInfo(term uint64) *node.GroupInfo {
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
func (s *LocalClientNodeInfoStorage) GetNodeInfo(term uint64, nodeId uint64) *node.NodeInfo {
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
