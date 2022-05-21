package router

import (
	"ecos/utils/logger"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"github.com/rcrowley/go-metrics"
	timeout "github.com/vearne/gin-timeout"
	"net/http"
	"time"
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
	timeoutMsg, _ := xml.Marshal(RequestTimeout(nil))
	router.Use(timeout.Timeout(
		timeout.WithTimeout(time.Minute),
		timeout.WithErrorHttpCode(http.StatusRequestTimeout),
		timeout.WithDefaultMsg(string(timeoutMsg)),
		timeout.WithCallBack(func(r *http.Request) {
			logger.Warningf("timeout happen, url: %s", r.URL.String())
		})))
	router.Use(func(c *gin.Context) {
		c.Header("Server", "ECOS")
		c.Header("Accept-Ranges", "bytes")
	})
	router.GET("/", listBuckets)
	router.HEAD("/", getMetrics)
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
	return router
}

func getMetrics(c *gin.Context) {
	c.Status(http.StatusOK)
	metrics.WriteJSONOnce(metrics.DefaultRegistry, c.Writer)
}
