package router

import (
	"ecos/utils/errno"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type InitiateMultipartUploadResult struct {
	Bucket   string `xml:"Bucket"`
	Key      string `xml:"Key"`
	UploadId string `xml:"UploadId"`
}

// parsePartID parses the part ID from the given header.
func parsePartID(partID string) (int, error) {
	id, err := strconv.Atoi(partID)
	if err != nil {
		return 0, err
	}
	if id < 1 || id > 10000 {
		return 0, errors.New("invalid part id")
	}
	return id, nil
}

// createMultipartUpload creates a multipart upload
func createMultipartUpload(c *gin.Context) {
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
	uploadId := Client.GetIOFactory(bucketName).CreateMultipartUploadJob(key)
	c.Header("Server", "Ecos")
	c.XML(http.StatusOK, InitiateMultipartUploadResult{
		Bucket:   bucketName,
		Key:      key,
		UploadId: uploadId,
	})
}

// uploadPart uploads a part
func uploadPart(c *gin.Context) {
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
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingUploadId.Error()})
		return
	}
	partID, err := parsePartID(c.Query("partNumber"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidPartId.Error()})
		return
	}
	if c.Request.ContentLength == 0 || c.Request.Body == http.NoBody {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.EmptyField.Error()})
		return
	}
	if c.Request.ContentLength > 5*1024*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.FileTooLarge.Error()})
		return
	}
	writer, err := Client.GetIOFactory(bucketName).GetMultipartUploadWriter(uploadId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidUploadId.Error()})
		return
	}
	eTag, err := writer.WritePart(partID, c.Request.Body)
	if err != nil {
		return
	}
	c.Header("Server", "Ecos")
	c.Header("ETag", eTag)
	c.Status(http.StatusOK)
}

type CompletedPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}

type CompleteMultipartUpload struct {
	Parts []CompletedPart `xml:"Part"`
}

type CompleteMultipartUploadResult struct {
	Location string `xml:"Location"`
	Bucket   string `xml:"Bucket"`
	Key      string `xml:"Key"`
	ETag     string `xml:"ETag"`
}

// completeMultipartUpload completes a multipart upload
func completeMultipartUpload(c *gin.Context) {
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
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingUploadId.Error()})
		return
	}
	var completeRequest CompleteMultipartUpload
	err := xml.NewDecoder(c.Request.Body).Decode(&completeRequest)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.EmptyField.Error()})
		return
	}
	etag, err := Client.GetIOFactory(bucketName).CompleteMultipartUploadJob(uploadId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.XML(http.StatusOK, CompleteMultipartUploadResult{
		Location: fmt.Sprintf("%s/%s", bucketName, key),
		Bucket:   bucketName,
		Key:      key,
		ETag:     etag,
	})
	c.Header("Server", "Ecos")
}

// abortMultipartUpload aborts a multipart upload
func abortMultipartUpload(c *gin.Context) {
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
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingUploadId.Error()})
		return
	}
	err := Client.GetIOFactory(bucketName).AbortMultipartUploadJob(uploadId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Server", "Ecos")
	c.Status(http.StatusOK)
}
