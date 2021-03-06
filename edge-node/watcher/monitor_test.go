package watcher

import (
	"context"
	"ecos/messenger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	nodeNum := 9
	defer cancel()
	basePath := "./ecos-data/monitor_test"
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

	for status := watchers[leader].Monitor.GetAllNodeReports(); len(status) != len(watchers); {
		time.Sleep(time.Second)
	}

	status := watchers[leader].Monitor.GetAllNodeReports()
	for _, s := range status {
		t.Logf("%v", s)
	}

	clusterReports, err := watchers[leader].Monitor.GetClusterReport(ctx, &emptypb.Empty{})
	assert.NoError(t, err)
	t.Logf("%v", clusterReports)

	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
}
