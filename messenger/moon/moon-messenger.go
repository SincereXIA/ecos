package moon

import (
	"context"
	"github.com/coreos/etcd/raft/raftpb"
)

type Server struct {
	UnimplementedMoonServer
	RaftChan chan raftpb.Message
}

func (s *Server) SendRaftMessage(_ context.Context, message *raftpb.Message) (*raftpb.Message, error) {
	s.RaftChan <- *message
	return &raftpb.Message{}, nil
}
