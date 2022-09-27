package watcher

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
	"os"
	"testing"
	"time"
)

func TestMonitor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	nodeNum := 5
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
	t.Logf("wait all test watcher OK!")

	leader := -1
	for i, w := range watchers {
		if w.GetMoon().IsLeader() {
			t.Logf("%v is leader", i)
			leader = i
			break
		}
	}
	assert.Greater(t, leader, -1)
	status := watchers[leader].Monitor.GetAllNodeReports()
	for len(status) != len(watchers) {
		t.Logf("node reports size miss match, len: %v", len(status))
		time.Sleep(time.Second)
		status = watchers[leader].Monitor.GetAllNodeReports()
	}

	for _, s := range status {
		t.Logf("%v", s)
	}

	clusterReports, err := watchers[leader].Monitor.GetClusterReport(ctx, &emptypb.Empty{})
	assert.NoError(t, err)
	t.Logf("%v", clusterReports)
	t.Logf("========= start test =======")

	t.Run("one server fail", func(t *testing.T) {
		id := (leader + 4) % nodeNum
		t.Logf("stop server: %v", watchers[id].selfNodeInfo.RpcPort)
		watchers[id].Stop()
		rpcServers[id].Server.GracefulStop()
		timer := time.NewTimer(time.Second * 10)

		for {
			ok := false
			select {
			case <-timer.C:
				t.Error("time out")
			default:
				clusterInfo := watchers[leader].GetCurrentClusterInfo()
				for _, nodeInfo := range clusterInfo.NodesInfo {
					if nodeInfo.RaftId == uint64(id+1) && nodeInfo.State == infos.NodeState_OFFLINE {
						t.Log(nodeInfo)
						ok = true
						break
					}
					if nodeInfo.RaftId != uint64(id+1) && nodeInfo.State == infos.NodeState_OFFLINE {
						t.Errorf("node: %v should not be offline", nodeInfo.RaftId)
					}
				}
			}
			if ok {
				t.Logf("detect one node fail: %v", id)
				break
			}
			time.Sleep(time.Second)
		}

	})

	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
}
