package demo

import (
	"ecos/messenger/auth"
	"golang.org/x/net/context"
)

type Server struct {
	UnimplementedGreeterServer
}

func (s *Server) SayHello(ctx context.Context, in *HelloRequest) (*HelloReply, error) {
	userID, err := auth.GetUserID(ctx)
	if err != nil {
		return nil, err
	}
	return &HelloReply{Message: "UserID: " + userID + "; Hello " + in.Name}, nil
}
