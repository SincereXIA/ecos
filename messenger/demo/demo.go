package demo

import (
	"golang.org/x/net/context"
)

type Server struct {
	UnimplementedGreeterServer
}

func (s *Server) SayHello(_ context.Context, in *HelloRequest) (*HelloReply, error) {
	return &HelloReply{Message: "Hello " + in.Name}, nil
}
