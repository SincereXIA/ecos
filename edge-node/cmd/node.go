package cmd

import (
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/config"
	"ecos/edge-node/gaia"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/watcher"
	gateway "ecos/gateway/router"
	"ecos/messenger"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"net/http"
	"strconv"
)

var nodeCmd = &cobra.Command{
	Use:   "node {run}",
	Short: "ecos edge node operate",
}

var nodeRunCmd = &cobra.Command{
	Use:   "run",
	Short: "start run edge node",
	Run:   nodeRun,
}

func init() {
	nodeRunCmd.Flags().StringP("sunAddr", "s", "", "ecos sun addr")
}

func nodeRun(cmd *cobra.Command, _ []string) {
	// read config
	confPath := cmd.Flag("config").Value.String()
	conf := config.DefaultConfig
	conf.WatcherConfig.SunAddr = cmd.Flag("sunAddr").Value.String()
	configUtil.Register(&conf, confPath)
	configUtil.ReadAll()

	// read history config
	_ = configUtil.GetConf(&conf)
	err := config.InitConfig(&conf)

	if err != nil {
		logger.Errorf("init config fail: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Gen Rpc
	rpc := messenger.NewRpcServer(conf.WatcherConfig.SelfNodeInfo.RpcPort)

	// Gen Moon
	storageFactory := infos.NewMemoryInfoFactory()
	storageRegisterBuilder := infos.NewStorageRegisterBuilder(storageFactory)
	storageRegister := storageRegisterBuilder.GetStorageRegister()
	m := moon.NewMoon(ctx, &conf.WatcherConfig.SelfNodeInfo, &conf.MoonConfig, rpc, storageRegister)

	// Gen Watcher
	w := watcher.NewWatcher(ctx, &conf.WatcherConfig, rpc, m, storageRegister)

	// Gen Alaya
	logger.Infof("Start init Alaya ...")
	//dbBasePath := path.Join(conf.StoragePath, "/db")
	//metaDBPath := path.Join(dbBasePath, "/meta")
	//metaStorage := alaya.NewStableMetaStorage(metaDBPath)
	metaStorageRegister := alaya.NewMemoryMetaStorageRegister()
	a := alaya.NewAlaya(ctx, w, &conf.AlayaConfig, metaStorageRegister, rpc)

	// Gen Gaia
	logger.Infof("Start init Gaia ...")
	_ = gaia.NewGaia(ctx, rpc, w, &conf.GaiaConfig)

	// Run
	go func() {
		err := rpc.Run()
		if err != nil {
			logger.Errorf("RPC server run error: %v", err)
		}
	}()
	go w.Run()
	go a.Run()
	go func() {
		// Gen Gateway
		logger.Infof("Start init Gateway ...")
		g := gateway.NewRouter(conf.GatewayConfig)
		pprof.Register(g)
		_ = g.Run()
	}()
	logger.Infof("edge node init success")

	// init Gin
	router := newRouter(w)
	port := strconv.FormatUint(conf.HttpPort, 10)
	_ = router.Run(":" + port)

	cancel()
}

func newRouter(w *watcher.Watcher) *gin.Engine {
	router := gin.Default()
	router.GET("/", getClusterInfo(w))
	return router
}

func getClusterInfo(w *watcher.Watcher) func(c *gin.Context) {
	return func(c *gin.Context) {
		if w == nil {
			c.String(http.StatusBadGateway, "Edge Node Not Ready")
		}
		clusterInfo := w.GetCurrentClusterInfo()
		info := clusterInfo.GetNodesInfo()
		c.JSONP(http.StatusOK, info)
	}
}
