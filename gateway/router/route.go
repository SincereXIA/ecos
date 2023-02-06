package router

import (
	"ecos/utils/logger"
	"github.com/gin-contrib/timeout"
	"github.com/gin-gonic/gin"
	"github.com/rcrowley/go-metrics"
	"net/http"
	"time"
)

const TIMEOUT = time.Minute

func timeoutMiddleware() gin.HandlerFunc {
	return timeout.New(
		timeout.WithTimeout(TIMEOUT),
		timeout.WithHandler(func(c *gin.Context) {
			c.Next()
		}),
	)
}

func NewRouter(cfg Config) *gin.Engine {
	if Client != nil {
		logger.Errorf("Client already initialized")
		return nil
	}
	clientConfig := cfg.ClientConfig
	//if cfg.Port != 0 {
	//	clientConfig.NodePort = cfg.Port
	//}
	InitClient(clientConfig)
	//router := gin.Default()
	router := gin.New()
	router.Use(gin.Logger()) // not recovery
	//router.Use(func(c *gin.Context) { // Use chan to record gateway process time
	//	startTime := time.Now()
	//	metricChan := make(chan string, 1)
	//	c.Set("metric", &metricChan)
	//	c.Next()
	//	select {
	//	case metricsName := <-metricChan:
	//		logger.Infof("Pushing to %s", metricsName)
	//		metrics.GetOrRegisterTimer(metricsName, nil).UpdateSince(startTime)
	//	default:
	//		logger.Infof("No metrics to push")
	//	}
	//	close(metricChan)
	//})
	router.Use(timeoutMiddleware())
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
