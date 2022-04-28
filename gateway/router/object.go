package router

import (
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// putObject creates a new object
func putObject(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKey.Error()})
		return
	}
	body := c.Request.Body
	writer := Client.GetIOFactory(bucketName).GetEcosWriter(key)
	_, err := io.Copy(&writer, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	err = writer.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Status(http.StatusOK)
}

// postObject creates a new object by post form
func postObject(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	mForm, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	key := mForm.Value["key"][0]
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errno.MissingKey.Error(),
		})
		return
	}
	file := mForm.File["file"][0]
	if file == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": errno.EmptyField.Error(),
		})
		return
	}
	content, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	writer := Client.GetIOFactory(bucketName).GetEcosWriter(key)
	_, err = io.Copy(&writer, content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	err = writer.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Status(http.StatusOK)
}

// headObject gets an object meta
func headObject(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKey.Error()})
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Get Object Operator
	op, err = op.Get(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	info, err := op.Info()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	meta := info.(*object.ObjectMeta)
	c.Header("Content-Length", strconv.FormatUint(meta.ObjSize, 10))
	c.Header("ETag", meta.ObjHash)
	c.Header("Last-Modified", meta.UpdateTime.Format(time.RFC850))
	c.Header("Server", "Ecos")
	c.Status(http.StatusOK)
}

// getObject gets an object
func getObject(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKey.Error()})
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Get Object Operator
	op, err = op.Get(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	info, err := op.Info()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	meta := info.(*object.ObjectMeta)
	reader := Client.GetIOFactory(bucketName).GetEcosReader(key)
	c.DataFromReader(http.StatusOK, int64(meta.ObjSize), "application/octet-stream", reader, nil)
}

// deleteObject deletes an object
func deleteObject(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKey.Error()})
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Delete Object from Bucket Operator
	err = op.Remove(key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Header("Content-Length", "0")
	c.Header("Server", "Ecos")
	c.Status(http.StatusNoContent)
}

type CopyObjectResult struct {
	ETag *string `xml:"ETag"`
}

func parseSrcKey(src string) (bucket, key string, err error) {
	if strings.HasPrefix(src, "/") {
		src = src[1:]
	}
	if strings.HasSuffix(src, "/") {
		src = src[:len(src)-1]
	}
	if strings.Contains(src, "/") {
		result := strings.SplitN(src, "/", 2)
		bucket = result[0]
		key = result[1]
	} else {
		bucket = src
	}
	return
}

// copyObject copies an object
func copyObject(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKey.Error()})
		return
	}
	src := c.GetHeader("x-amz-copy-source")
	if src == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	bucket, srcKey, err := parseSrcKey(src)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidArgument.Error()})
		return
	}
	if bucket == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKey.Error()})
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Get Object Operator
	op, err = op.Get(srcKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	info, err := op.Info()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	meta := info.(*object.ObjectMeta)
	writer := Client.GetIOFactory(bucketName).GetEcosWriter(key)
	etag, err := writer.Copy(meta)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Header("Server", "Ecos")
	c.XML(http.StatusOK, CopyObjectResult{ETag: etag})
}

type ObjectIdentifier struct {
	Key *string `xml:"Key"`
	// VersionId *string `xml:"VersionId"`
}

type DeletedObject struct {
	// DeleteMarker *string `xml:"DeleterMarker"`
	// DeleteMarkerVersion *string `xml:"DeleterMarkerVersion"`
	Key *string `xml:"Key"`
	// VersionId *string `xml:"VersionId"`
}

type DeleteError struct {
	Code    *string `xml:"Code"`
	Key     *string `xml:"Key"`
	Message *string `xml:"Message"`
	// VersionId *string `xml:"VersionId"`
}

type Delete struct {
	Object []ObjectIdentifier `xml:"Object"`
	Quiet  *bool              `xml:"Quiet"`
}

type DeleteResult struct {
	Deleted []DeletedObject `xml:"Deleted"`
	Error   []DeleteError   `xml:"Error"`
}

// deleteObjects deletes multiple objects
func deleteObjects(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	body := c.Request.Body
	var deleteRequest Delete
	err := xml.NewDecoder(body).Decode(&deleteRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	var deleteResult DeleteResult
	internalErr := "InternalError"
	for _, obj := range deleteRequest.Object {
		// Delete Objects from Bucket Operator
		err = op.Remove(*obj.Key)
		if err != nil {
			errMsg := err.Error()
			deleteResult.Error = append(deleteResult.Error, DeleteError{
				Code:    &internalErr,
				Key:     obj.Key,
				Message: &errMsg,
			})
		} else {
			deleteResult.Deleted = append(deleteResult.Deleted, DeletedObject(obj))
		}
	}
	c.Header("Server", "Ecos")
	c.XML(http.StatusOK, deleteResult)
}

// objectLevelPostHandler handles object level POST requests
// POST /{bucketName}/{key} Include:
//  CreateMultiPartUpload:   POST /Key+?uploads
//  CompleteMultipartUpload: POST /Key+?uploadId=UploadId
func objectLevelPostHandler(c *gin.Context) {
	if _, ok := c.GetQuery("uploads"); ok {
		createMultipartUpload(c)
		return
	}
	if _, ok := c.GetQuery("uploadId"); ok {
		completeMultipartUpload(c)
		return
	}
	c.Status(http.StatusNotFound)
}

// objectLevelPutHandler handles object level PUT requests
//
// PUT /{bucketName}/{key} Include:
//  PutObject:  PUT /Key+
//  CopyObject: PUT /Key+
//  UploadPart: PUT /Key+?uploadId=UploadId&partNumber=PartNumber
func objectLevelPutHandler(c *gin.Context) {
	if _, ok := c.GetQuery("uploadId"); ok {
		uploadPart(c)
		return
	}
	if c.GetHeader("x-amz-copy-source") != "" {
		copyObject(c)
		return
	}
	putObject(c)
}

// objectLevelGetHandler handles object level GET requests
//
// GET /{bucketName}/{key} Include:
//  GetObject: GET /Key+?partNumber=PartNumber&response-cache-control=ResponseCacheControl&response-content-disposition=ResponseContentDisposition&response-content-encoding=ResponseContentEncoding&response-content-language=ResponseContentLanguage&response-content-type=ResponseContentType&response-expires=ResponseExpires&versionId=VersionId
//  ListParts: GET /Key+?max-parts=MaxParts&part-number-marker=PartNumberMarker&uploadId=UploadId
func objectLevelGetHandler(c *gin.Context) {
	if _, ok := c.GetQuery("uploadId"); ok {
		listParts(c)
		return
	}
	if _, ok := c.GetQuery("partNumber"); ok {
		getPart(c)
		return
	}
	getObject(c)
}

// objectLevelDeleteHandler handles object level DELETE requests
//
// DELETE /{bucketName}/{key} Include:
//  DeleteObject:         DELETE /Key+?versionId=VersionId
//  AbortMultipartUpload: DELETE /Key+?uploadId=UploadId
func objectLevelDeleteHandler(c *gin.Context) {
	if _, ok := c.GetQuery("uploadId"); ok {
		abortMultipartUpload(c)
	}
	deleteObject(c)
}
