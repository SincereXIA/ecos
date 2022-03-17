package demo

import (
	"fmt"
	"net"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	UnimplementedGreeterServer
}

func (s *Server) SayHello(_ context.Context, in *HelloRequest) (*HelloReply, error) {
	return &HelloReply{Message: "Hello " + in.Name}, nil
}

func ServerRun() {
	// 监听本地的8972端口
	lis, err := net.Listen("tcp", ":32671")
	if err != nil {
		fmt.Printf("failed to listen %v", err.Error())
		return
	}
	s := grpc.NewServer()               // 创建gRPC服务器
	RegisterGreeterServer(s, &Server{}) // 在gRPC服务端注册服务

	reflection.Register(s) //在给定的gRPC服务器上注册服务器反射服务
	// Serve方法在lis上接受传入连接，为每个连接创建一个ServerTransport和server的goroutine。
	// 该goroutine读取gRPC请求，然后调用已注册的处理程序来响应它们。
	err = s.Serve(lis)
	if err != nil {
		fmt.Printf("failed to serve: %v", err)
		return
	}
}
