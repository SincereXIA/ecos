package watcher

import (
	"context"
	"ecos/messenger"
	"testing"
)

func TestNewWatcher(t *testing.T) {
	basePath := "./ecos-data"
	nodeNum := 9
	ctx := context.Background()
	// Run Sun

	watchers, rpcServers, _ := GenTestWatcherCluster(ctx, basePath, nodeNum)

	for i := 0; i < nodeNum; i++ {
		go func(rpc *messenger.RpcServer) {
			err := rpc.Run()
			if err != nil {
				t.Errorf("rpc server run error: %v", err)
			}
		}(rpcServers[i])
	}

	RunAllTestWatcher(watchers)
	WaitAllTestWatcherOK(watchers)
}

func TestNewWatcherMockMoon(t *testing.T) {
	basePath := "./ecos-data"
	nodeNum := 9
	ctx := context.Background()

	watchers, rpcServers, _, controllers := GenMockWatcherCluster(t, ctx, basePath, nodeNum)

	for i := 0; i < nodeNum; i++ {
		go func(rpc *messenger.RpcServer) {
			err := rpc.Run()
			if err != nil {
				t.Errorf("rpc server run error: %v", err)
			}
		}(rpcServers[i])
	}

	RunAllTestWatcher(watchers)
	WaitAllTestWatcherOK(watchers)

	for i := 0; i < nodeNum; i++ {
		controllers[i].Finish()
	}
}
