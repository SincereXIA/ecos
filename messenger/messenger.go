package messenger

import (
	"ecos/edge-node/infos"
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
	ListenPort uint64
	listen     net.Listener
}

func NewRpcServer(listenPort uint64) *RpcServer {
	s := grpc.NewServer() // 创建gRPC服务器
	demo.RegisterGreeterServer(s, &demo.Server{})
	lis, err := net.Listen("tcp", ":"+strconv.FormatUint(listenPort, 10))
	if err != nil {
		logger.Errorf("RpcServer run at: %v error: %v", listenPort, err)
	}
	server := &RpcServer{
		Server:     s,
		ListenPort: listenPort,
		listen:     lis,
	}
	return server
}

// NewRandomPortRpcServer return a new RpcServer with random port signed by os,
// this should only be used in test. to avoid port conflict.
func NewRandomPortRpcServer() (port uint64, server *RpcServer) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.Errorf("RpcServer run at random port error: %v", err)
	}
	port = uint64(lis.Addr().(*net.TCPAddr).Port)
	logger.Warningf("[TEST ONLY] RpcServer create at random port: %v", port)
	s := grpc.NewServer() // 创建gRPC服务器
	server = &RpcServer{
		Server:     s,
		ListenPort: port,
		listen:     lis,
	}
	return
}

func (server *RpcServer) Run() error {
	if server.listen == nil {
		logger.Errorf("RpcServer run at %v fail, listener is nil", server.ListenPort)
		return nil
	}
	reflection.Register(server) //在给定的gRPC服务器上注册服务器反射服务
	// Serve方法在lis上接受传入连接，为每个连接创建一个ServerTransport和server的goroutine。
	// 该goroutine读取gRPC请求，然后调用已注册的处理程序来响应它们。
	logger.Infof("RpcServer running at: %v", server.ListenPort)
	err := server.Serve(server.listen)
	if err != nil {
		logger.Errorf("RpcServer run at: %v error: %v", server.ListenPort, err)
		_ = server.listen.Close()
		return err
	}
	return nil
}

func (server *RpcServer) Stop() {
	logger.Infof("RpcServer stop: %v", server.ListenPort)
	server.Server.Stop()
}

func GetRpcConn(addr string, port uint64) (*grpc.ClientConn, error) {
	strPort := strconv.FormatUint(port, 10)
	conn, err := grpc.Dial(addr+":"+strPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	return conn, err
}

func GetRpcConnByNodeInfo(info *infos.NodeInfo) (*grpc.ClientConn, error) {
	addr := info.IpAddr
	port := info.RpcPort
	return GetRpcConn(addr, port)
}
