package main

import (
	"ecos/cloud/sun"
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
	// init Sun
	logger.Infof("Start init Sun ...")
	rpcServer := messenger.NewRpcServer(3267)
	s := sun.NewSun(rpcServer)
	go rpcServer.Run()

	Sun = s
	logger.Infof("Sun init success")

	// init Gin
	router := NewRouter()
	_ = router.Run(":3268")
}
