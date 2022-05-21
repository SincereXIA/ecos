package router

import (
	"ecos/client/credentials"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/utils/common"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type CreateBucketConfiguration struct {
	LocationConstraint *string `xml:"locationConstraint"`
}

// createBucket creates a new bucket
func createBucket(c *gin.Context) {
	bucketName, err := parseBucket(c)
	if err != nil {
		return
	}
	if bucketName == "default" {
		c.XML(http.StatusConflict, BucketAlreadyExists("default"))
		return
	}
	if c.Request.ContentLength > 0 {
		var createBucketConfiguration CreateBucketConfiguration
		err := xml.NewDecoder(c.Request.Body).Decode(&createBucketConfiguration)
		if err != nil {
			c.XML(http.StatusBadRequest, MalformedXML(bucketName, c.Request.URL.Path, nil))
			return
		}
	}
	volOp := Client.GetVolumeOperator()
	credential := credentials.Credential{
		AccessKey: "",
		SecretKey: "",
	}
	bucketInfo := infos.GenBucketInfo(credential.GetUserID(), bucketName, credential.GetUserID())
	err = volOp.CreateBucket(bucketInfo)
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
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
	listObjectsResult, bucketName, err := listBucket(c)
	if err != nil {
		return
	}
	result := ListBucketResult{
		Name: &bucketName,
	}
	for _, meta := range listObjectsResult {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
			return
		}
		timestamp := meta.UpdateTime.Format(time.RFC3339Nano)
		result.Contents = append(result.Contents, Content{
			Key:          &key,
			LastModified: &timestamp,
			ETag:         &meta.ObjId,
			Size:         meta.ObjSize,
		})
	}
	c.XML(http.StatusOK, result)
}

// listObjectsV2 lists objects
func listObjectsV2(c *gin.Context) {
	listObjectsResult, bucketName, err := listBucket(c)
	if err != nil {
		return
	}
	result := ListBucketResult{
		Name: &bucketName,
	}
	for _, meta := range listObjectsResult {
		_, _, key, _, err := object.SplitID(meta.ObjId)
		if err != nil {
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
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
	_, _, err := getBucketOperator(c)
	if err != nil {
		return
	}
	c.Status(http.StatusOK)
}

// deleteBucket deletes an EMPTY bucket
//
// DELETE /{bucketName}
func deleteBucket(c *gin.Context) {
	credential := credentials.Credential{
		AccessKey: "",
		SecretKey: "",
	}
	listObjectsResult, bucketName, err := listBucket(c)
	if err != nil {
		return
	}
	if len(listObjectsResult) != 0 {
		c.XML(http.StatusConflict, BucketNotEmpty(bucketName))
		return
	}
	bucketInfo := infos.GenBucketInfo(credential.GetUserID(), bucketName, credential.GetUserID())
	err = Client.GetVolumeOperator().DeleteBucket(bucketInfo)
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
		return
	}
	c.Status(http.StatusNoContent)
}

type Bucket struct {
	Name         *string `xml:"Name"`
	CreationDate *string `xml:"CreationDate"`
}

type Owner struct {
	ID          *string `xml:"ID"`
	DisplayName *string `xml:"DisplayName"`
}

type ListAllMyBucketsResult struct {
	Owner   *Owner `xml:"Owner"`
	Buckets *struct {
		Buckets []Bucket `xml:"Bucket"`
	} `xml:"Buckets"`
}

// listBucket lists all buckets
//
// GET /
func listBuckets(c *gin.Context) {
	var credential credentials.Credential
	bucketOps, err := Client.GetVolumeOperator().List("")
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), "", c.Request.URL.Path, nil))
		return
	}
	result := ListAllMyBucketsResult{
		Owner: &Owner{
			ID:          common.PtrString(credential.GetUserID()),
			DisplayName: common.PtrString(credential.GetUserID()),
		},
		Buckets: &struct {
			Buckets []Bucket `xml:"Bucket"`
		}{},
	}
	for _, op := range bucketOps {
		rawInfo, err := op.Info()
		if err != nil {
			continue
		}
		bucketInfo := rawInfo.(*infos.BucketInfo)
		formattedTime := time.Now().Format(time.RFC3339)
		result.Buckets.Buckets = append(result.Buckets.Buckets, Bucket{
			Name:         &bucketInfo.BucketName,
			CreationDate: &formattedTime,
		})
	}
	c.XML(http.StatusOK, result)
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

// bucketLevelDeleteHandler handles bucket level DELETE requests
//
// DELETE /{bucketName} Include:
//  DeleteBucket: DELETE /
func bucketLevelDeleteHandler(c *gin.Context) {
	deleteBucket(c)
}
