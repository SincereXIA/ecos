package router

import (
	"ecos/client/credentials"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

// TODO: DeleteBucket
//	 ShowBucketStat

type CreateBucketConfiguration struct {
	LocationConstraint *string `xml:"locationConstraint"`
}

// createBucket creates a new bucket
func createBucket(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	if c.Request.ContentLength > 0 {
		var createBucketConfiguration CreateBucketConfiguration
		err := xml.NewDecoder(c.Request.Body).Decode(&createBucketConfiguration)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	volOp := Client.GetVolumeOperator()
	credential := credentials.Credential{
		AccessKey: "",
		SecretKey: "",
	}
	bucketInfo := infos.GenBucketInfo(credential.GetUserID(), bucketName, credential.GetUserID())
	err := volOp.CreateBucket(bucketInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Location", "/"+bucketName)
	c.Status(http.StatusOK)
}

type Content struct {
	Key          *string `xml:"Key"`
	LastModified *string `xml:"LastModified"`
	ETag         *string `xml:"ETag"`
	Size         uint64  `xml:"Size"`
	// StorageClass *string `xml:"StorageClass"`
	// Owner struct {
	//	 ID          *string `xml:"ID"`
	//	 DisplayName *string `xml:"DisplayName"`
	// } `xml:"Owner"`
}

type ListBucketResult struct {
	Name        *string   `xml:"Name"`
	Prefix      *string   `xml:"Prefix"`
	Marker      *string   `xml:"Marker"`
	MaxKeys     int       `xml:"MaxKeys"`
	IsTruncated bool      `xml:"IsTruncated"`
	Contents    []Content `xml:"Contents"`
}

// listObjects lists objects
func listObjects(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": errno.MissingBucket.Error(),
		})
	}
	listObjectsResult, err := Client.ListObjects(c, bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"message": errno.BucketNotFound.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	result := ListBucketResult{
		Name: &bucketName,
	}
	for _, meta := range listObjectsResult {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		timestamp := meta.UpdateTime.Format(time.RFC3339Nano)
		result.Contents = append(result.Contents, Content{
			Key:          &key,
			LastModified: &timestamp,
			ETag:         &meta.ObjHash,
			Size:         meta.ObjSize,
		})
	}
	c.XML(http.StatusOK, result)
}

// listObjectsV2 lists objects
func listObjectsV2(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errno.MissingBucket.Error(),
		})
		return
	}
	listObjectsResult, err := Client.ListObjects(c, bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"message": errno.BucketNotFound.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	result := ListBucketResult{
		Name: &bucketName,
	}
	for _, meta := range listObjectsResult {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		timestamp := meta.UpdateTime.Format(time.RFC3339Nano)
		result.Contents = append(result.Contents, Content{
			Key:          &key,
			LastModified: &timestamp,
			ETag:         &meta.ObjHash,
			Size:         meta.ObjSize,
		})
	}
	c.XML(http.StatusOK, result)
}

// headBucket checks if the bucket exists
//
// HEAD /{bucketName}
func headBucket(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"code": http.StatusNotFound,
		})
		return
	}
	_, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"message": errno.BucketNotFound.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Status(http.StatusOK)
	return
}

// bucketLevelPostHandler handles bucket level POST requests
//
// POST /{bucketName} Include:
//  PostObject:   POST /
//  DeleteObject: POST /?delete
func bucketLevelPostHandler(c *gin.Context) {
	if _, del := c.GetQuery("delete"); del {
		deleteObjects(c)
		return
	}
	postObject(c)
}

// bucketLevelGetHandler handles bucket level GET requests
//
// GET /{bucketName} Include:
//  ListObjects:          GET /?delimiter=Delimiter&encoding-type=EncodingType&marker=Marker&max-keys=MaxKeys&prefix=Prefix
//  ListObjectsV2:        GET /?list-type=2&continuation-token=ContinuationToken&delimiter=Delimiter&encoding-type=EncodingType&fetch-owner=FetchOwner&max-keys=MaxKeys&prefix=Prefix&start-after=StartAfter
//  ListMultipartUploads: GET /?uploads&delimiter=Delimiter&encoding-type=EncodingType&key-marker=KeyMarker&max-uploads=MaxUploads&prefix=Prefix&upload-id-marker=UploadIdMarker
func bucketLevelGetHandler(c *gin.Context) {
	if _, uploads := c.GetQuery("uploads"); uploads {
		listMultipartUploads(c)
		return
	}
	if listType := c.Query("list-type"); listType == "2" {
		listObjectsV2(c)
		return
	}
	listObjects(c)
}

// bucketLevelPutHandler handles bucket level PUT requests
//
// PUT /{bucketName} Include:
//  CreateBucket: PUT /
func bucketLevelPutHandler(c *gin.Context) {
	createBucket(c)
}

// bucketLevelHeadHandler handles bucket level HEAD requests
//
// HEAD /{bucketName} Include:
//  HeadBucket: HEAD /
func bucketLevelHeadHandler(c *gin.Context) {
	headBucket(c)
}
