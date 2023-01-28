package watcher

import (
	"context"
	"ecos/cloud/config"
	"ecos/cloud/rainbow"
	"ecos/cloud/sun"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/messenger"
	"ecos/utils/logger"
	"github.com/golang/mock/gomock"
	"path"
	"strconv"
	"strings"
	"time"
)

func GenTestWatcher(ctx context.Context, basePath string, sunAddr string) (*Watcher, *messenger.RpcServer) {
	moonConfig := moon.DefaultConfig
	moonConfig.RaftStoragePath = path.Join(basePath, "moon")
	moonConfig.RocksdbStoragePath = path.Join(basePath, "rocksdb")
	port, nodeRpc := messenger.NewRandomPortRpcServer()
	nodeInfo := infos.NewSelfInfo(0, "127.0.0.1", port)
	builder := infos.NewStorageRegisterBuilder(infos.NewRocksDBInfoStorageFactory(path.Join(basePath, "rocksdb")))
	register := builder.GetStorageRegister()
	m := moon.NewMoon(ctx, nodeInfo, &moonConfig, nodeRpc, register)

	watcherConfig := DefaultConfig
	watcherConfig.SunAddr = sunAddr
	watcherConfig.SelfNodeInfo = *nodeInfo
	cloudAddr := strings.Split(sunAddr, ":")[0]
	cloudPort, _ := strconv.Atoi(strings.Split(sunAddr, ":")[1])
	watcherConfig.CloudAddr = cloudAddr
	watcherConfig.CloudPort = uint64(cloudPort)

	return NewWatcher(ctx, &watcherConfig, nodeRpc, m, register), nodeRpc
}

func GenTestMockWatcher(t gomock.TestReporter, ctx context.Context,
	register *infos.StorageRegister, sunAddr string, isLeader bool) (*gomock.Controller, *Watcher, *messenger.RpcServer) {
	port, nodeRpc := messenger.NewRandomPortRpcServer()
	nodeInfo := infos.NewSelfInfo(0, "127.0.0.1", port)

	mockCtrl := gomock.NewController(t)
	testMoon := moon.NewMockInfoController(mockCtrl)
	moon.InitMock(testMoon, nodeRpc, register, nodeInfo, isLeader)

	watcherConfig := DefaultConfig
	watcherConfig.SunAddr = sunAddr
	watcherConfig.SelfNodeInfo = *nodeInfo
	watcherConfig.NodeInfoCommitInterval = time.Second * 2
	cloudAddr := strings.Split(sunAddr, ":")[0]
	cloudPort, _ := strconv.Atoi(strings.Split(sunAddr, ":")[1])
	watcherConfig.CloudAddr = cloudAddr
	watcherConfig.CloudPort = uint64(cloudPort)

	return mockCtrl, NewWatcher(ctx, &watcherConfig, nodeRpc, testMoon, register), nodeRpc
}

func GenMockWatcherCluster(t gomock.TestReporter, ctx context.Context, _ string, num int) ([]*Watcher, []*messenger.RpcServer, string, []*gomock.Controller) {
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

	// init rainbow
	logger.Infof("Start init rainbow ...")
	_ = rainbow.NewRainbow(ctx, sunRpc, &config.DefaultCloudConfig)

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
	timer := time.NewTimer(time.Minute)
	for i := 0; i < clusterNodeNum; i++ {
		err := watchers[i].ctx.Err()
		if err != nil {
			clusterNodeNum -= 1
			continue
		}
		for {
			select {
			case <-timer.C:
				logger.Errorf("WaitAllTestWatcherOK timeout")
				return
			default:
			}
			if watchers[i].moon.GetLeaderID() <= 0 {
				logger.Debugf("WaitAllTestWatcherOK wait leader, node: %v, leader: %v",
					watchers[i].GetSelfInfo().RaftId, watchers[i].moon.GetLeaderID())
				time.Sleep(time.Millisecond * 300)
			} else {
				logger.Debugf("WaitAllTestWatcherOK get leader, node: %v", watchers[i].GetSelfInfo().RaftId)
				break
			}
		}
	}
	logger.Debugf("WaitAllTestWatcherOK get leader, start check health")
	for {
		ok := true
		select {
		case <-timer.C:
			logger.Errorf("WaitAllTestWatcherOK timeout")
			return
		default:
		}
		for _, w := range watchers {
			err := w.ctx.Err()
			if err != nil { // 跳过
				continue
			}
			clusterInfo := w.GetCurrentClusterInfo()
			healthNode := clusterInfo.GetHealthNode()
			if len(healthNode) < clusterNodeNum {
				ok = false
				logger.Debugf("WaitAllTestWatcherOK wait health node, node: %v, health node num: %v", w.GetSelfInfo().RaftId, len(healthNode))
				break
			}
			for _, n := range healthNode {
				if n.RaftId == w.GetSelfInfo().RaftId {
					if n.IpAddr != w.GetSelfInfo().IpAddr || n.RpcPort != w.GetSelfInfo().RpcPort {
						ok = false
						logger.Debugf("WaitAllTestWatcherOK wait health node, node: %v", w.GetSelfInfo().RaftId)
						break
					}
				}
			}
		}
		if ok {
			return
		}
		time.Sleep(time.Millisecond * 300)
	}
}
