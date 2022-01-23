package messenger

import (
	"ecos/messenger/demo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"strconv"
)

type RpcServer struct {
	*grpc.Server
	listenPort uint64
}

func NewRpcServer(listenPort uint64) *RpcServer {
	s := grpc.NewServer() // 创建gRPC服务器
	demo.RegisterGreeterServer(s, &demo.Server{})
	return &RpcServer{s, listenPort}
}

func (server *RpcServer) Run() error {
	// 监听本地的8972端口
	lis, err := net.Listen("tcp", ":"+strconv.FormatUint(server.listenPort, 10))
	if err != nil {
		return err
	}
	reflection.Register(server) //在给定的gRPC服务器上注册服务器反射服务
	// Serve方法在lis上接受传入连接，为每个连接创建一个ServerTransport和server的goroutine。
	// 该goroutine读取gRPC请求，然后调用已注册的处理程序来响应它们。
	err = server.Serve(lis)
	if err != nil {
		return err
	}
	return nil
}
