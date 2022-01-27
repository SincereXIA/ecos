package main

import (
	moon "ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/utils/logger"
	"github.com/gin-gonic/gin"
	"net/http"
)

var Moon *moon.Moon

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

func main() {
	// init moon node
	logger.Infof("Start init moon node ...")
	selfInfo := node.GetSelfInfo()
	rpcServer := messenger.NewRpcServer(3267)
	m := moon.NewMoon(selfInfo, "127.0.0.1:3267", nil, nil, // Todo: sun info
		rpcServer)
	go rpcServer.Run()
	go m.Run()
	Moon = m
	logger.Infof("moon node init success")

	// init Gin
	router := NewRouter()
	_ = router.Run(":3268")
}
