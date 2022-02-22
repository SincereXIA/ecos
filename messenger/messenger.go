package messenger

import (
	"ecos/messenger/demo"
	"ecos/utils/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"net"
	"strconv"
)

type RpcServer struct {
	*grpc.Server
	listenPort uint64
	listen     net.Listener
}

func NewRpcServer(listenPort uint64) *RpcServer {
	s := grpc.NewServer() // 创建gRPC服务器
	demo.RegisterGreeterServer(s, &demo.Server{})
	return &RpcServer{s, listenPort, nil}
}

func (server *RpcServer) Run() error {
	// 监听本地的8972端口
	lis, err := net.Listen("tcp", ":"+strconv.FormatUint(server.listenPort, 10))
	server.listen = lis
	if err != nil {
		logger.Errorf("RpcServer run at: %v error: %v", server.listenPort, err)
		return err
	}
	reflection.Register(server) //在给定的gRPC服务器上注册服务器反射服务
	// Serve方法在lis上接受传入连接，为每个连接创建一个ServerTransport和server的goroutine。
	// 该goroutine读取gRPC请求，然后调用已注册的处理程序来响应它们。
	logger.Infof("RpcServer running at: %v", server.listenPort)
	err = server.Serve(lis)
	if err != nil {
		logger.Errorf("RpcServer run at: %v error: %v", server.listenPort, err)
		server.listen.Close()
		return err
	}
	return nil
}

func (server *RpcServer) Stop() {
	logger.Infof("RpcServer stop: %v", server.listenPort)
	server.Server.Stop()
}

func GetRpcConn(addr string, port uint64) (*grpc.ClientConn, error) {
	strPort := strconv.FormatUint(port, 10)
	conn, err := grpc.Dial(addr+":"+strPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	return conn, err
}
