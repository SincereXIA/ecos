package watcher

import (
	"context"
	"ecos/messenger"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	nodeNum := 9
	defer cancel()
	basePath := "./ecos-data/"
	watchers, rpcServers, _ := GenTestWatcherCluster(ctx, basePath, nodeNum)
	for _, rpc := range rpcServers {
		go func(rpc *messenger.RpcServer) {
			err := rpc.Run()
			if err != nil {
				t.Errorf("rpc server failed: %v", err)
			}
		}(rpc)
	}
	RunAllTestWatcher(watchers)
	WaitAllTestWatcherOK(watchers)

	leader := -1
	for i, w := range watchers {
		if w.GetMoon().IsLeader() {
			leader = i
			break
		}
	}
	assert.Greater(t, leader, -1)

	for status := watchers[leader].monitor.GetAllReports(); len(status) != len(watchers); {
		time.Sleep(time.Second)
	}

	status := watchers[leader].monitor.GetAllReports()
	for _, s := range status {
		t.Logf("%v", s)
	}

	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
}

func genTestMonitors(ctx context.Context, watchers []*Watcher, rpcServers []*messenger.RpcServer) []Monitor {
	monitors := make([]Monitor, len(watchers))
	for i, w := range watchers {
		monitors[i] = NewMonitor(ctx, w, rpcServers[i])
	}
	return monitors
}
