package router

import (
	"bytes"
	"ecos/utils/errno"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type InitiateMultipartUploadResult struct {
	Bucket   *string `xml:"Bucket"`
	Key      *string `xml:"Key"`
	UploadId *string `xml:"UploadId"`
}

// parsePartID parses the part ID from the given header.
func parsePartID(partID string) (int32, error) {
	id, err := strconv.Atoi(partID)
	if err != nil {
		return 0, err
	}
	if id < 1 || id > 10000 {
		return 0, errors.New("invalid part id")
	}
	return int32(id), nil
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
		Bucket:   &bucketName,
		Key:      &key,
		UploadId: &uploadId,
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

type ListPartsResult struct {
	Bucket   *string      `xml:"Bucket"`
	Key      *string      `xml:"Key"`
	UploadId *string      `xml:"UploadId"`
	Part     []types.Part `xml:"Part"`
}

// listParts lists all parts
func listParts(c *gin.Context) {
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
	writer, err := Client.GetIOFactory(bucketName).GetMultipartUploadWriter(uploadId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidUploadId.Error()})
		return
	}
	parts := writer.ListParts()
	c.Header("Server", "Ecos")
	c.XML(http.StatusOK, ListPartsResult{
		Bucket:   &bucketName,
		Key:      &key,
		UploadId: &uploadId,
		Part:     parts,
	})
}

// getPart gets a parts of a multipart upload
func getPart(c *gin.Context) {
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
	writer, err := Client.GetIOFactory(bucketName).GetMultipartUploadWriter(uploadId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidUploadId.Error()})
		return
	}
	blockInfo, err := writer.GetPartBlockInfo(partID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidPartId.Error()})
		return
	}
	reader := Client.GetIOFactory(bucketName).GetEcosReader(key)
	content, err := reader.GetBlock(blockInfo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err})
		return
	}
	if uint64(len(content)) != blockInfo.BlockSize {
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": errno.IncompatibleSize.Error()})
		return
	}
	fullReader := bytes.NewReader(content)
	c.Header("Server", "Ecos")
	c.DataFromReader(http.StatusOK, int64(blockInfo.BlockSize), "application/octet-stream", fullReader, nil)
}

type CompletedPart struct {
	PartNumber int32   `xml:"PartNumber"`
	ETag       *string `xml:"ETag"`
}

type CompleteMultipartUpload struct {
	Parts []CompletedPart `xml:"Part"`
}

type CompleteMultipartUploadResult struct {
	Location *string `xml:"Location"`
	Bucket   *string `xml:"Bucket"`
	Key      *string `xml:"Key"`
	ETag     *string `xml:"ETag"`
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
	location := fmt.Sprintf("%s/%s", bucketName, key)
	c.XML(http.StatusOK, CompleteMultipartUploadResult{
		Location: &location,
		Bucket:   &bucketName,
		Key:      &key,
		ETag:     &etag,
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

type ListMultipartUploadsResult struct {
	Bucket             *string                 `xml:"Bucket"`
	KeyMarker          *string                 `xml:"KeyMarker"`
	UploadIdMarker     *string                 `xml:"UploadIdMarker"`
	NextKeyMarker      *string                 `xml:"NextKeyMarker"`
	NextUploadIdMarker *string                 `xml:"NextUploadIdMarker"`
	Delimiter          *string                 `xml:"Delimiter"`
	Prefix             *string                 `xml:"Prefix"`
	MaxUploads         int                     `xml:"MaxUploads"`
	IsTruncated        *bool                   `xml:"IsTruncated"`
	Uploads            []types.MultipartUpload `xml:"Upload"`
}

// listMultipartUploads lists all multipart uploads
func listMultipartUploads(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.MissingBucket.Error()})
		return
	}
	prefix := c.Query("prefix")
	//keyMarker := c.Query("keyMarker")
	//if keyMarker == "" {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingKeyMarker.Error()})
	//	return
	//}
	//uploadIdMarker := c.Query("uploadIdMarker")
	//if uploadIdMarker == "" {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": errno.MissingUploadIdMarker.Error()})
	//	return
	//}
	delimiter := c.Query("delimiter")
	maxUploads, err := strconv.Atoi(c.Query("maxUploads"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidArgument.Error()})
		return
	}
	if maxUploads < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": errno.InvalidArgument.Error()})
		return
	}
	uploads, err := Client.GetIOFactory(bucketName).ListMultipartUploadJob()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	isTruncated := false
	ret := ListMultipartUploadsResult{
		Bucket:      &bucketName,
		IsTruncated: &isTruncated,
		Uploads:     uploads,
	}
	if maxUploads > 0 {
		ret.MaxUploads = maxUploads
	}
	if delimiter != "" {
		ret.Delimiter = &delimiter
	}
	if prefix != "" {
		ret.Prefix = &prefix
	}
	c.XML(http.StatusOK, ret)
}
