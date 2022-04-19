package monitor

import (
	"context"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	basePath := "./ecos-data/"
	watchers, rpcServers, _ := watcher.GenTestWatcherCluster(ctx, basePath, 9)
	monitors := genTestMonitors(ctx, watchers, rpcServers)
	for i, rpc := range rpcServers {
		go func(rpc *messenger.RpcServer) {
			err := rpc.Run()
			if err != nil {
				t.Errorf("rpc server failed: %v", err)
			}
		}(rpc)
		go monitors[i].Run()
	}
	watcher.RunAllTestWatcher(watchers)
	watcher.WaitAllTestWatcherOK(watchers)

	leader := -1
	for i, w := range watchers {
		if w.GetMoon().IsLeader() {
			leader = i
			break
		}
	}
	assert.Greater(t, leader, -1)

	for status := monitors[leader].GetAllReports(); len(status) != len(watchers); {
		time.Sleep(time.Second)
	}

	status := monitors[leader].GetAllReports()
	for _, s := range status {
		t.Logf("%v", s)
	}
}

func genTestMonitors(ctx context.Context, watchers []*watcher.Watcher, rpcServers []*messenger.RpcServer) []Monitor {
	monitors := make([]Monitor, len(watchers))
	for i, w := range watchers {
		monitors[i] = NewMonitor(ctx, w, rpcServers[i])
	}
	return monitors
}
