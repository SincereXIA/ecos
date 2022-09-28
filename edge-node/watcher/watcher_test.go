package watcher

import (
	"context"
	"ecos/edge-node/infos"
	moon2 "ecos/edge-node/moon"
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
	basePath := "./ecos-data/watcher-test"
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

	moon := watchers[0].moon
	bucket := infos.GenBucketInfo("test", "default", "test")
	_, err := moon.ProposeInfo(ctx, &moon2.ProposeInfoRequest{
		Operate:  moon2.ProposeInfoRequest_ADD,
		Id:       bucket.GetID(),
		BaseInfo: bucket.BaseInfo(),
	})
	if err != nil {
		t.Errorf("propose bucket error: %v", err)
	}

	t.Cleanup(func() {
		for i := 0; i < nodeNum; i++ {
			watchers[i].moon.Stop()
			rpcServers[i].Stop()
		}
	})

	t.Run("remove a node", func(t *testing.T) {
		watchers[nodeNum-1].moon.Stop()
		watchers[nodeNum-1].cancelFunc()
		rpcServers[nodeNum-1].Stop()
		WaitAllTestWatcherOK(watchers)
	})
}
