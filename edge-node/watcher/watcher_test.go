package watcher

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	moon2 "ecos/shared/moon"
	"ecos/utils/config"
	"github.com/stretchr/testify/assert"
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
	// Run First half
	firstRunNum := nodeNum/2 + 1

	RunAllTestWatcher(watchers[:firstRunNum])
	WaitAllTestWatcherOK(watchers[:firstRunNum])

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

	t.Run("add left half", func(t *testing.T) {
		RunAllTestWatcher(watchers[firstRunNum:])
		WaitAllTestWatcherOK(watchers)
		// test node weight
		clusterInfo := watchers[0].GetCurrentClusterInfo()
		if config.GlobalSharedConfig.Behave == config.BehaveMapX {
			for i, node := range clusterInfo.NodesInfo {
				if i < firstRunNum {
					assert.Equal(t, node.Capacity, uint64(0))
				} else {
					assert.Greaterf(t, node.Capacity, uint64(0), "node %d capacity is 0", i)
				}
			}
			// Gen pipeline
			blockPipe := pipeline.GenBlockPipelines(clusterInfo)
			for _, pipe := range blockPipe {
				t.Logf("block pipe: %v", pipe)
			}

		} else {
			for i, node := range clusterInfo.NodesInfo {
				assert.Greaterf(t, node.Capacity, uint64(0), "node %d capacity is 0", i)
			}
		}
	})

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
		WaitAllTestWatcherOK(watchers[:nodeNum-1])
	})
}
