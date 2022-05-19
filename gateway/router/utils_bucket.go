package router

import (
	"ecos/client"
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

/* Bucket Section of Route Utils */

func getBucketOperator(c *gin.Context) (operator *client.BucketOperator, bucketName string, err error) {
	bucketName, err = parseBucket(c)
	if err != nil {
		return
	}
	ret, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
	}
	operator = ret.(*client.BucketOperator)
	return
}

func listBucket(c *gin.Context) (listObjectsResult []*object.ObjectMeta, bucketName string, err error) {
	bucketName, err = parseBucket(c)
	if err != nil {
		return
	}
	prefix := c.Query("prefix")
	listObjectsResult, err = Client.ListObjects(c, bucketName, prefix)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
		return
	}
	return
}
