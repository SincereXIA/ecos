package watcher

import (
	"context"
	"ecos/messenger"
	"testing"
)

func TestWatcher(t *testing.T) {
	t.Run("watcher with real moon", func(t *testing.T) {
		testWatcher(t, false)
	})
	t.Run("watcher with mock moon", func(t *testing.T) {
		testWatcher(t, true)
	})
}

func testWatcher(t *testing.T, mock bool) {
	basePath := "./ecos-data"
	nodeNum := 9
	ctx := context.Background()
	var watchers []*Watcher
	var rpcServers []*messenger.RpcServer
	// Run Sun
	if mock {
		watchers, rpcServers, _, _ = GenMockWatcherCluster(t, ctx, basePath, nodeNum)
	} else {
		watchers, rpcServers, _ = GenTestWatcherCluster(ctx, basePath, nodeNum)
	}

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

	t.Cleanup(func() {
		for i := 0; i < nodeNum; i++ {
			watchers[i].moon.Stop()
			rpcServers[i].Stop()
		}
	})
}
