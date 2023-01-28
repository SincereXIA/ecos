package cmd

import (
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/config"
	"ecos/edge-node/gaia"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/outpost"
	"ecos/edge-node/watcher"
	gateway "ecos/gateway/router"
	"ecos/messenger"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strconv"
	"syscall"
	"time"
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
	defer cancel()

	// Print config
	logger.Infof("[Conf] behave: ", conf.SharedConfig.Behave)

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
	dbBasePath := path.Join(conf.StoragePath, "/db")
	metaDBPath := path.Join(dbBasePath, "/meta")
	metaStorageRegister, err := alaya.NewRocksDBMetaStorageRegister(metaDBPath)
	//metaStorageRegister := alaya2.NewMemoryMetaStorageRegister()
	a := alaya.NewAlaya(ctx, w, &conf.AlayaConfig, metaStorageRegister, rpc)

	// Gen Gaia
	logger.Infof("Start init Gaia ...")
	_ = gaia.NewGaia(ctx, rpc, w, &conf.GaiaConfig)

	// Gen Outpost
	logger.Infof("Start init Outpost ...")
	o, err := outpost.NewOutpost(ctx, conf.WatcherConfig.SunAddr, w)
	if err != nil {
		logger.Errorf("init outpost fail: %v", err)
	}

	// Run
	go func() {
		err := rpc.Run()
		if err != nil {
			logger.Errorf("RPC server run error: %v", err)
		}
	}()
	go w.Run()
	go a.Run()
	go o.Run()
	go func() {
		// Gen Gateway
		logger.Infof("Start init Gateway ...")
		g := gateway.NewRouter(conf.GatewayConfig)
		_ = g.Run()
	}()
	logger.Infof("edge node init success")

	// 监听系统信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			logger.Infof("receive signal from os: %v, ECOS node start stop", s)
			cancel()
			time.Sleep(time.Second)
			os.Exit(0)
		}
	}()

	// init Gin
	router := newRouter(w)
	port := strconv.FormatUint(conf.HttpPort, 10)
	_ = router.Run(":" + port)
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
