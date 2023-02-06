package main

import (
	"context"
	config2 "ecos/client/config"
	"ecos/cloud/config"
	"ecos/cloud/rainbow"
	"ecos/cloud/sun"
	gateway "ecos/gateway/router"
	"ecos/messenger"
	"ecos/utils/logger"
	"github.com/gin-gonic/gin"
	"net/http"
)

var Sun *sun.Sun

func NewRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/", hello)
	return router
}

func hello(c *gin.Context) {
	c.String(http.StatusOK, "ECOS cloud running")
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init Sun
	logger.Infof("Start init Sun ...")
	rpcServer := messenger.NewRpcServer(3267)
	s := sun.NewSun(rpcServer)
	go func() {
		err := rpcServer.Run()
		if err != nil {
			logger.Errorf("Sun rpc server run err: %v", err)
		}
	}()

	Sun = s
	logger.Infof("Sun init success")

	// init rainbow
	logger.Infof("Start init rainbow ...")
	_ = rainbow.NewRainbow(ctx, rpcServer, &config.DefaultCloudConfig)

	// init gateway
	logger.Infof("Start init gateway ...")
	gatewayConf := gateway.DefaultConfig
	gatewayConf.ClientConfig.ConnectType = config2.ConnectCloud
	g := gateway.NewRouter(gatewayConf)
	_ = g.Run()

	// init Gin
	router := NewRouter()
	_ = router.Run(":3268")
}
