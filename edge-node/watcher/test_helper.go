package watcher

import (
	"context"
	"ecos/cloud/sun"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/messenger"
	"ecos/utils/logger"
	"path"
	"strconv"
	"time"
)

func GenTestWatcher(ctx context.Context, basePath string, sunAddr string) (*Watcher, *messenger.RpcServer) {
	moonConfig := moon.DefaultConfig
	moonConfig.RaftStoragePath = path.Join(basePath, "moon")
	port, nodeRpc := messenger.NewRandomPortRpcServer()
	nodeInfo := infos.NewSelfInfo(0, "127.0.0.1", port)
	builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
	register := builder.GetStorageRegister()
	m := moon.NewMoon(ctx, nodeInfo, &moonConfig, nodeRpc, builder.GetStorageRegister())

	watcherConfig := DefaultConfig
	watcherConfig.SunAddr = sunAddr
	watcherConfig.SelfNodeInfo = *nodeInfo

	return NewWatcher(ctx, &watcherConfig, nodeRpc, m, register), nodeRpc
}

func GenTestWatcherCluster(ctx context.Context, basePath string, num int) ([]*Watcher, []*messenger.RpcServer, string) {
	sunPort, sunRpc := messenger.NewRandomPortRpcServer()
	sun.NewSun(sunRpc)
	go func() {
		err := sunRpc.Run()
		if err != nil {
			logger.Errorf("Run rpcServer err: %v", err)
		}
	}()
	sunAddr := "127.0.0.1:" + strconv.FormatUint(sunPort, 10)
	time.Sleep(1 * time.Second)

	var watchers []*Watcher
	var rpcServers []*messenger.RpcServer
	for i := 0; i < num; i++ {
		watcher, rpc := GenTestWatcher(ctx, path.Join(basePath, strconv.Itoa(i)), sunAddr)
		watchers = append(watchers, watcher)
		rpcServers = append(rpcServers, rpc)
	}
	return watchers, rpcServers, sunAddr
}

func RunAllTestWatcher(watchers []*Watcher) {
	for _, w := range watchers {
		w.Run()
	}
}

func WaitAllTestWatcherOK(watchers []*Watcher) {
	clusterNodeNum := len(watchers)
	for {
		ok := true
		for _, w := range watchers {
			info := w.GetCurrentClusterInfo()
			if len(info.NodesInfo) != clusterNodeNum {
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(time.Millisecond * 300)
	}
}
