package router

import (
	"ecos/edge-node/object"
	"github.com/gin-gonic/gin"
)

// TODO: Create Bucket
//   DeleteBucket
//	 ShowBucketStat
//   ListMultipartUploads

type Content struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         uint64 `xml:"Size"`
	// StorageClass string `xml:"StorageClass"`
	// Owner struct {
	//	 ID          string `xml:"ID"`
	//	 DisplayName string `xml:"DisplayName"`
	// } `xml:"Owner"`
}

type ListBucketResult struct {
	Name        string    `xml:"Name"`
	Prefix      string    `xml:"Prefix"`
	Marker      string    `xml:"Marker"`
	MaxKeys     int       `xml:"MaxKeys"`
	IsTruncated bool      `xml:"IsTruncated"`
	Contents    []Content `xml:"Contents"`
}

// listObjects lists objects
func listObjects(c *gin.Context) {
	apiVersion := c.Query("list-type")
	if apiVersion == "2" {
		listObjectsV2(c)
		return
	}
	bucketName := c.Param("bucketName")
	listObjectsResult, err := Client.ListObjects(c, bucketName)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	result := ListBucketResult{
		Name: bucketName,
	}
	for _, meta := range listObjectsResult {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
		result.Contents = append(result.Contents, Content{
			Key:          key,
			LastModified: meta.UpdateTime.Format("2000-01-01T00:00:00.000Z"),
			ETag:         meta.ObjHash,
			Size:         meta.ObjSize,
		})
	}
	c.XML(200, result)
}

// listObjectsV2 lists objects
func listObjectsV2(c *gin.Context) {
	bucketName := c.Param("bucketName")
	listObjectsResult, err := Client.ListObjects(c, bucketName)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	result := ListBucketResult{
		Name: bucketName,
	}
	for _, meta := range listObjectsResult {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}
		result.Contents = append(result.Contents, Content{
			Key:          key,
			LastModified: meta.UpdateTime.Format("2000-01-01T00:00:00.000Z"),
			ETag:         meta.ObjHash,
			Size:         meta.ObjSize,
		})
	}
	c.XML(200, result)
}

// bucketLevelPostHandler handles bucket level requests
//
// POST /:bucketName - POST Object
//
// POST /:bucketName?delete - DeleteObjects
func bucketLevelPostHandler(c *gin.Context) {
	_, del := c.GetQuery("delete")
	if del {
		deleteObjects(c)
		return
	}
	postObject(c)
}
