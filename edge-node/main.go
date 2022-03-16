package main

import (
	"ecos/edge-node/alaya"
	edgeNodeConfig "ecos/edge-node/config"
	"ecos/edge-node/gaia"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/utils/config"
	"ecos/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/urfave/cli/v2"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

var Moon *moon.Moon
var Alaya *alaya.Alaya
var Gaia *gaia.Gaia

func NewRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/", getGroupInfo)
	return router
}

func getGroupInfo(c *gin.Context) {
	if Moon == nil {
		c.String(http.StatusBadGateway, "Edge Node Not Ready")
	}
	info := Moon.InfoStorage.ListAllNodeInfo()
	c.JSONP(http.StatusOK, info)
}

func action(c *cli.Context) error {
	// read config
	confPath := c.Path("config")
	conf := edgeNodeConfig.DefaultConfig
	conf.Moon.SunAddr = c.String("sun")
	config.Register(conf, confPath)
	config.ReadAll()
	_ = config.GetConf(conf)
	err := edgeNodeConfig.InitConfig(conf)
	if err != nil {
		logger.Errorf("init config fail: %v", err)
	}
	dbBasePath := path.Join(conf.StoragePath, "/db")

	// init moon node
	logger.Infof("Start init moon node ...")
	nodeInfoDBPath := path.Join(dbBasePath, "/nodeInfo")
	infoStorage := node.NewStableNodeInfoStorage(nodeInfoDBPath)
	selfInfo := conf.SelfInfo
	rpcServer := messenger.NewRpcServer(conf.RpcPort)
	stableStorage := moon.NewStorage(path.Join(dbBasePath, "/raft/moon"))
	Moon = moon.NewMoon(selfInfo, conf.Moon.SunAddr, nil, nil,
		rpcServer, infoStorage, stableStorage)

	//init Alaya
	logger.Infof("Start init Alaya ...")
	metaDBPath := path.Join(dbBasePath, "/meta")
	metaStorage := alaya.NewStableMetaStorage(metaDBPath)
	Alaya = alaya.NewAlaya(selfInfo, infoStorage, metaStorage, rpcServer)

	//init Gaia
	logger.Infof("Start init Gaia ...")
	Gaia = gaia.NewGaia(rpcServer, selfInfo, infoStorage, gaia.DefaultConfig)

	// Run
	go func() {
		err := rpcServer.Run()
		if err != nil {
			logger.Errorf("RPC server run error: %v", err)
		}
	}()
	go Moon.Run()
	go Alaya.Run()

	logger.Infof("moon node init success")

	// init Gin
	router := NewRouter()
	port := strconv.FormatUint(conf.HttpPort, 10)
	_ = router.Run(":" + port)
	return nil
}

func main() {
	// init cli
	flags := []cli.Flag{
		&cli.PathFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "set config file",
			Value:   "./edge_node.json",
		},
		&cli.StringFlag{
			Name:    "sun",
			Aliases: []string{"s"},
			Usage:   "set ecos-sun addr",
			Value:   "",
		},
	}
	app := cli.App{
		Name:                 "ECOS-EdgeNode",
		Usage:                "",
		UsageText:            "",
		ArgsUsage:            "",
		Version:              "",
		Flags:                flags,
		EnableBashCompletion: true,
		BashComplete:         nil,
		Action:               action,
		Compiled:             time.Time{},
		Authors: []*cli.Author{{
			Name:  "Zhang Junhua",
			Email: "zhangjh@mail.act.buaa.edu.cn",
		}},
		Copyright: "Copyright BUAA 2022",
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Errorf("edge node run error: %v", err)
	}

}
