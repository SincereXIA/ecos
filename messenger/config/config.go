package config

import (
	"context"
	"ecos/messenger/tags"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"google.golang.org/grpc"
)

type Config struct {
	// The address of the server.
	ListenAddr  string
	ListenPort  int
	AuthEnabled bool
}

var DefaultConfig = Config{
	AuthEnabled: false,
	ListenPort:  0,
	ListenAddr:  "",
}

// SetGrpcConfig set grpc config to context by tags
func SetGrpcConfig(config Config) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		grpc_ctxtags.Extract(ctx).Set(tags.TagRpcConfig, config)
		return handler(ctx, req)
	}
}

// GetGrpcConfig get grpc config from context by tags
func GetGrpcConfig(ctx context.Context) Config {
	return grpc_ctxtags.Extract(ctx).Values()[tags.TagRpcConfig].(Config)
}
