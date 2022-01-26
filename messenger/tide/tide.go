package tide

import (
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Server struct {
	UnimplementedNodeTideServer
}

func (s *Server) GetClusterStatus(_ context.Context, _ *emptypb.Empty) (*ClusterStatus, error) {
	var nodes []*ClusterStatus_Node
	for i := 0; i < 4; i++ {
		nodes[i] = &ClusterStatus_Node{
			UUID:        uuid.New().String(),
			IpAddr:      "127.0.0.1",
			StoragePath: "/",
			Capacity:    uint64(i * 1024),
		}
	}
	return &ClusterStatus{
		Nodes: nodes,
	}, nil
}
