package watcher

import (
	"context"
	"ecos/cloud/sun"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/messenger"
	"ecos/utils/logger"
	"github.com/golang/mock/gomock"
	"path"
	"strconv"
	"testing"
	"time"
)

func GenTestWatcher(ctx context.Context, basePath string, sunAddr string) (*Watcher, *messenger.RpcServer) {
	moonConfig := moon.DefaultConfig
	moonConfig.RaftStoragePath = path.Join(basePath, "moon")
	port, nodeRpc := messenger.NewRandomPortRpcServer()
	nodeInfo := infos.NewSelfInfo(0, "127.0.0.1", port)
	builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
	register := builder.GetStorageRegister()
	m := moon.NewMoon(ctx, nodeInfo, &moonConfig, nodeRpc, register)

	watcherConfig := DefaultConfig
	watcherConfig.SunAddr = sunAddr
	watcherConfig.SelfNodeInfo = *nodeInfo

	return NewWatcher(ctx, &watcherConfig, nodeRpc, m, register), nodeRpc
}

func GenTestMockWatcher(t *testing.T, ctx context.Context,
	register *infos.StorageRegister, sunAddr string, isLeader bool) (*gomock.Controller, *Watcher, *messenger.RpcServer) {
	port, nodeRpc := messenger.NewRandomPortRpcServer()
	nodeInfo := infos.NewSelfInfo(0, "127.0.0.1", port)

	mockCtrl := gomock.NewController(t)
	testMoon := moon.NewMockInfoController(mockCtrl)
	moon.InitMock(testMoon, nodeRpc, register, nodeInfo, isLeader)

	watcherConfig := DefaultConfig
	watcherConfig.SunAddr = sunAddr
	watcherConfig.SelfNodeInfo = *nodeInfo
	watcherConfig.NodeInfoCommitInterval = time.Millisecond * 100

	return mockCtrl, NewWatcher(ctx, &watcherConfig, nodeRpc, testMoon, register), nodeRpc
}

func GenMockWatcherCluster(t *testing.T, ctx context.Context, _ string, num int) ([]*Watcher, []*messenger.RpcServer, string, []*gomock.Controller) {
	sunPort, sunRpc := messenger.NewRandomPortRpcServer()
	sun.NewSun(sunRpc)
	go func() {
		err := sunRpc.Run()
		if err != nil {
			logger.Errorf("Run rpcServer err: %v", err)
		}
	}()
	builder := infos.NewStorageRegisterBuilder(infos.NewMemoryInfoFactory())
	register := builder.GetStorageRegister()
	sunAddr := "127.0.0.1:" + strconv.FormatUint(sunPort, 10)

	for len(sunRpc.GetServiceInfo()) == 0 {
		time.Sleep(time.Millisecond * 10)
	}

	var watchers []*Watcher
	var rpcServers []*messenger.RpcServer
	var controllers []*gomock.Controller
	for i := 0; i < num; i++ {
		isLeader := false
		if i == 0 {
			isLeader = true
		}
		controller, watcher, rpc := GenTestMockWatcher(t, ctx, register, sunAddr, isLeader)
		watchers = append(watchers, watcher)
		rpcServers = append(rpcServers, rpc)
		controllers = append(controllers, controller)
	}

	return watchers, rpcServers, sunAddr, controllers
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
