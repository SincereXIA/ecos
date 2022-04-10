package edgeNodeTest

import (
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/gaia"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/logger"
	"path"
	"strconv"
	"testing"
	"time"
)

func RunTestEdgeNodeCluster(t *testing.T, ctx context.Context, mock bool,
	basePath string, num int) ([]*watcher.Watcher, []*messenger.RpcServer) {
	var watchers []*watcher.Watcher
	var rpcServers []*messenger.RpcServer
	if mock {
		watchers, rpcServers, _, _ = watcher.GenMockWatcherCluster(t, ctx, basePath, num)
	} else {
		watchers, rpcServers, _ = watcher.GenTestWatcherCluster(ctx, basePath, num)
	}
	alayas := GenAlayaCluster(ctx, basePath, watchers, rpcServers)
	_ = GenGaiaCluster(ctx, basePath, watchers, rpcServers)

	for i := 0; i < num; i++ {
		go func(server *messenger.RpcServer) {
			err := server.Run()
			if err != nil {
				logger.Errorf("Run rpc server fail: %v", err)
			}
		}(rpcServers[i])
	}
	time.Sleep(100 * time.Millisecond)
	watcher.RunAllTestWatcher(watchers)

	for _, a := range alayas {
		go a.Run()
	}
	waiteAllAlayaOK(alayas)
	return watchers, rpcServers
}

func GenAlayaCluster(ctx context.Context, basePath string, watchers []*watcher.Watcher, rpcServers []*messenger.RpcServer) []alaya.Alayaer {
	var alayas []alaya.Alayaer
	nodeNum := len(watchers)
	for i := 0; i < nodeNum; i++ {
		// TODO (qiutb): implement rocksdb MetaStorage
		//metaStorage := alaya.NewStableMetaStorage(path.Join(basePath, strconv.Itoa(i), "alaya", "meta"))
		metaStorage := alaya.NewMemoryMetaStorage()
		a := alaya.NewAlaya(ctx, watchers[i], metaStorage, rpcServers[i])
		alayas = append(alayas, a)
	}
	return alayas
}

func GenGaiaCluster(ctx context.Context, basePath string, watchers []*watcher.Watcher, rpcServers []*messenger.RpcServer) []*gaia.Gaia {
	nodeNum := len(watchers)
	var gaias []*gaia.Gaia
	for i := 0; i < nodeNum; i++ {
		config := gaia.Config{BasePath: path.Join(basePath, "gaia", strconv.Itoa(i)), ChunkSize: 1 << 20}
		g := gaia.NewGaia(ctx, rpcServers[i], watchers[i], &config)
		gaias = append(gaias, g)
	}
	return gaias
}

func waiteAllAlayaOK(alayas []alaya.Alayaer) {
	timer := time.After(60 * time.Second)
	for {
		select {
		case <-timer:
			logger.Warningf("Alayas not OK after time out")
			for _, a := range alayas {
				switch x := a.(type) {
				case *alaya.Alaya:
					x.PrintPipelineInfo()
				}
			}
			return
		default:
		}
		ok := true
		for _, a := range alayas {
			if !a.IsAllPipelinesOK() {
				//logger.Warningf("Alaya %v not ok", id+1)
				ok = false
				break
			}
		}
		if ok {
			return
		}
		time.Sleep(time.Millisecond * 200)
	}
}
