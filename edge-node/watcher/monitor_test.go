package watcher

import (
	"context"
	"ecos/messenger"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	nodeNum := 9
	defer cancel()
	basePath := "./ecos-data/"
	watchers, rpcServers, _ := GenTestWatcherCluster(ctx, basePath, nodeNum)
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

	for status := monitors[leader].GetAllReports(); len(status) != len(watchers); {
		time.Sleep(time.Second)
	}

	status := monitors[leader].GetAllReports()
	for _, s := range status {
		t.Logf("%v", s)
	}

	t.Run("monitor stop", func(t *testing.T) {
		c := monitors[leader].GetEventChannel()
		monitors[nodeNum-1].Stop()
		timer := time.NewTimer(time.Second * 5)
		select {
		case event := <-c:
			t.Logf("get node %v stop event", event.Report.NodeId)
			assert.Equal(t, uint64(nodeNum), event.Report.NodeId)
		case <-timer.C:
			t.Errorf("monitor not find node stopped")
		}
	})
}

func genTestMonitors(ctx context.Context, watchers []*Watcher, rpcServers []*messenger.RpcServer) []Monitor {
	monitors := make([]Monitor, len(watchers))
	for i, w := range watchers {
		monitors[i] = NewMonitor(ctx, w, rpcServers[i])
	}
	return monitors
}
