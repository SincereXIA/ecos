package messenger

import (
	"ecos/edge-node/infos"
	"ecos/messenger/auth"
	"ecos/messenger/config"
	"ecos/utils/logger"
	"errors"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	lru "github.com/hashicorp/golang-lru"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"net"
	"strconv"
)

type RpcServer struct {
	*grpc.Server
	ListenPort uint64
	listen     net.Listener
}

func NewRpcServer(listenPort uint64) *RpcServer {
	conf := config.DefaultConfig
	conf.ListenPort = int(listenPort)
	return NewRpcServerWithConfig(conf)
}

func NewRpcServerWithConfig(conf config.Config) *RpcServer {
	listenPort := conf.ListenPort
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(listenPort))
	if err != nil {
		logger.Errorf("RpcServer run at: %v error: %v", listenPort, err)
	}
	port := uint64(lis.Addr().(*net.TCPAddr).Port)
	conf.ListenPort = int(port)
	// 创建gRPC服务器
	var unaryInterceptors []grpc.UnaryServerInterceptor
	unaryInterceptors = append(unaryInterceptors, grpc_ctxtags.UnaryServerInterceptor())
	unaryInterceptors = append(unaryInterceptors, config.SetGrpcConfig(conf))
	if conf.AuthEnabled {
		unaryInterceptors = append(unaryInterceptors,
			grpc_auth.UnaryServerInterceptor(auth.BasicAuthFunc))
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptors...)))
	server := &RpcServer{
		Server:     s,
		ListenPort: port,
		listen:     lis,
	}
	return server
}

// NewRandomPortRpcServer return a new RpcServer with random port signed by os,
// this should only be used in test. to avoid port conflict.
func NewRandomPortRpcServer() (port uint64, server *RpcServer) {
	server = NewRpcServerWithConfig(config.DefaultConfig)
	port = server.ListenPort
	logger.Warningf("[TEST ONLY] RpcServer create at random port: %v", port)
	return port, server
}

func (server *RpcServer) Run() error {
	if server.listen == nil {
		logger.Errorf("RpcServer run at %v fail, listener is nil", server.ListenPort)
		return nil
	}
	//reflection.Register(server) //在给定的gRPC服务器上注册服务器反射服务
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

var lruPool *lru.Cache

func GetRpcConn(addr string, port uint64) (*grpc.ClientConn, error) {
	addrWithPort := genAddrWithPort(addr, port)
	if value, ok := lruPool.Get(addrWithPort); ok {
		conn := value.(*grpc.ClientConn)
		if conn.GetState() == connectivity.Shutdown {
			logger.Warningf("GetRpcConn: %v is shutdown, recreate it", addrWithPort)
		} else {
			return value.(*grpc.ClientConn), nil
		}
	}
	conn, err := grpc.Dial(addrWithPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Errorf("GetRpcConn error: %v", err)
		return nil, err
	}
	lruPool.Add(addrWithPort, conn)
	return conn, err
}

func GetRpcConnByNodeInfo(info *infos.NodeInfo) (*grpc.ClientConn, error) {
	addr := info.IpAddr
	port := info.RpcPort
	if info.State == infos.NodeState_OFFLINE {
		return nil, errors.New("node offline")
	}
	return GetRpcConn(addr, port)
}

func genAddrWithPort(addr string, port uint64) string {
	return addr + ":" + strconv.FormatUint(port, 10)
}

func init() {
	var err error
	lruPool, err = lru.NewWithEvict(64, func(key interface{}, value interface{}) {
		switch conn := value.(type) {
		case *grpc.ClientConn:
			_ = conn.Close()
		}
	})
	if err != nil {
		logger.Fatalf("init rpc conn lruPool error: %v", err)
	}
}
