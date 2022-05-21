package router

import (
	"ecos/utils/logger"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/rcrowley/go-metrics"
	"net"
	"net/http"
)

func NewRouter(cfg Config) *gin.Engine {
	if Client != nil {
		logger.Errorf("Client already initialized")
		return nil
	}
	clientConfig := cfg.ClientConfig
	if cfg.Host != "" {
		clientConfig.NodeAddr = cfg.Host
	}
	if cfg.Port != 0 {
		clientConfig.NodePort = cfg.Port
	}
	InitClient(clientConfig)
	router := gin.Default()
	router.GET("/", hello)
	router.GET("/metrics", getMetrics)
	// Bucket Routes
	bucketRouter := router.Group("/:bucketName")
	{
		bucketRouter.PUT("", bucketLevelPutHandler)
		bucketRouter.DELETE("", bucketLevelDeleteHandler)
		bucketRouter.GET("", bucketLevelGetHandler)
		bucketRouter.HEAD("", bucketLevelHeadHandler)
		bucketRouter.POST("", bucketLevelPostHandler)
	}
	// Object Routes
	{
		bucketRouter.PUT("/*key", objectLevelPutHandler)
		bucketRouter.DELETE("/*key", objectLevelDeleteHandler)
		bucketRouter.GET("/*key", objectLevelGetHandler)
		bucketRouter.HEAD("/*key", objectLevelHeadHandler)
		bucketRouter.POST("/*key", objectLevelPostHandler)
	}
	router.Use(func(c *gin.Context) {
		c.Header("Server", "ECOS")
		c.Header("Accept-Ranges", "bytes")
	})
	return router
}

func hello(c *gin.Context) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
		return
	}
	selfIp := ""
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				selfIp = ipnet.IP.String()
			}
		}
	}
	c.String(http.StatusOK, "ECOS EdgeNode: %s", selfIp)
}

func getMetrics(c *gin.Context) {
	c.Status(http.StatusOK)
	metrics.WriteJSONOnce(metrics.DefaultRegistry, c.Writer)
}
