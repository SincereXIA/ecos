package sun

import (
	"context"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/messenger/common"
	"sync"
	"sync/atomic"
)

// Sun used to help edge nodes become a group
type Sun struct {
	rpc *messenger.RpcServer
	Server
	leaderInfo *node.NodeInfo
	groupInfo  *node.GroupInfo
	lastRaftID uint64
	mu         sync.Mutex
	cachedInfo map[string]*node.NodeInfo //cache node info by uuid
}

type Server struct {
	UnimplementedSunServer
}

// MoonRegister give a Raft ID to a new edge node
func (s *Sun) MoonRegister(_ context.Context, nodeInfo *node.NodeInfo) (*RegisterResult, error) {
	hasLeader := true

	// Check Leader info
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.leaderInfo == nil { // This is new leader
		hasLeader = false
		s.leaderInfo = nodeInfo
		s.groupInfo.LeaderInfo = s.leaderInfo
	}

	if info, ok := s.cachedInfo[nodeInfo.Uuid]; ok {
		return &RegisterResult{
			Result: &common.Result{
				Status: common.Result_OK,
			},
			RaftId:    info.RaftId,
			HasLeader: hasLeader,
			GroupInfo: s.groupInfo,
		}, nil
	}

	// Gen a new Raft ID
	raftID := atomic.AddUint64(&s.lastRaftID, 1)
	nodeInfo.RaftId = raftID

	result := RegisterResult{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		RaftId:    raftID,
		HasLeader: hasLeader,
		GroupInfo: s.groupInfo,
	}

	s.cachedInfo[nodeInfo.Uuid] = nodeInfo
	return &result, nil
}

func (s *Sun) GetLeaderInfo(_ context.Context, nodeInfo *node.NodeInfo) (*node.NodeInfo, error) {
	return s.groupInfo.LeaderInfo, nil
}

func (s *Sun) ReportGroupInfo(_ context.Context, groupInfo *node.GroupInfo) (*common.Result, error) {
	s.mu.Lock()
	s.groupInfo = groupInfo
	s.mu.Unlock()
	result := common.Result{
		Status: common.Result_OK,
	}
	return &result, nil
}

func NewSun(rpc *messenger.RpcServer) *Sun {
	sun := Sun{
		rpc:        rpc,
		leaderInfo: nil,
		groupInfo: &node.GroupInfo{
			LeaderInfo: nil,
			NodesInfo:  nil,
		},
		lastRaftID: 0,
		mu:         sync.Mutex{},
		cachedInfo: map[string]*node.NodeInfo{},
	}
	RegisterSunServer(rpc, &sun)
	return &sun
}
