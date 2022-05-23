package router

import (
	"ecos/utils/errno"
	"github.com/gin-gonic/gin"
	"net/http"
)

/* Common Section of Route Utils */

func parseBucket(c *gin.Context) (bucketName string, err error) {
	bucketName = c.Param("bucketName")
	if bucketName == "" {
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		err = errno.InvalidArgument
	}
	return
}

func parseBucketKey(c *gin.Context) (bucketName, key string, err error) {
	bucketName, err = parseBucket(c)
	if err != nil {
		return
	}
	key = c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		err = errno.InvalidArgument
	}
	return
}

var metricsChan = make(chan string, 1)
