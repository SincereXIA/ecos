package watcher

import (
	"context"
	"ecos/cloud/sun"
	"ecos/edge-node/infos"
	moon "ecos/edge-node/moon"
	"ecos/messenger"
	"path"
	"strconv"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	basePath := "./ecos-data"
	nodeNum := 5
	ctx := context.Background()
	// Run Sun
	sunPort, sunRpc := messenger.NewRandomPortRpcServer()
	sun.NewSun(sunRpc)
	go func() {
		err := sunRpc.Run()
		if err != nil {
			t.Errorf("Run rpcServer err: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)

	var watchers []*Watcher
	for i := 0; i < nodeNum; i++ {
		moonConfig := moon.DefaultConfig
		port, nodeRpc := messenger.NewRandomPortRpcServer()
		nodeInfo := infos.NewSelfInfo(0, "127.0.0.1", port)
		builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
		raftStorage := moon.NewStorage(path.Join(basePath, "/raft", strconv.Itoa(i+1)))
		register := builder.GetStorageRegister()
		m := moon.NewMoon(nodeInfo, moonConfig, nodeRpc, builder.GetStorageRegister(), raftStorage)

		watcherConfig := DefaultConfig
		watcherConfig.SunAddr = "127.0.0.1:" + strconv.FormatUint(sunPort, 10)
		watcherConfig.SelfNodeInfo = *nodeInfo

		watchers = append(watchers, NewWatcher(ctx, &watcherConfig, nodeRpc, m, register))
		go nodeRpc.Run()
	}

	for i := 0; i < nodeNum; i++ {
		leaderInfo, err := watchers[i].AskSky()
		if err != nil {
			t.Errorf("watcher ask sky err: %v", err)
			return
		}
		err = watchers[i].RequestJoinCluster(leaderInfo)
		if err != nil {
			t.Errorf("watcher request join to cluster err: %v", err)
		}
		watchers[i].StartMoon()
	}
	time.Sleep(10 * time.Second)
}
