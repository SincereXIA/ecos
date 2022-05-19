package router

import (
	"bytes"
	"ecos/edge-node/object"
	"ecos/utils/common"
	"ecos/utils/errno"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	key := c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	uploadId := factory.CreateMultipartUploadJob(key)
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	key := c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("uploadId", bucketName, c.Request.URL.Path, nil))
		return
	}
	partID, err := parsePartID(c.Query("partNumber"))
	if err != nil {
		c.XML(http.StatusBadRequest, InvalidArgument("partNumber", bucketName, c.Request.URL.Path, nil))
		return
	}
	if c.Request.ContentLength < 5<<20 {
		c.XML(http.StatusBadRequest, EntityTooSmall(bucketName, c.Request.URL.Path, key))
		return
	}
	if c.Request.ContentLength > 5<<30 {
		c.XML(http.StatusBadRequest, EntityTooLarge(bucketName, c.Request.URL.Path, key))
		return
	}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	writer, err := factory.GetMultipartUploadWriter(uploadId)
	if err == errno.NoSuchUpload {
		c.XML(http.StatusNotFound, NoSuchUpload(bucketName, c.Request.URL.Path, key))
		return
	}
	eTag, err := writer.WritePart(partID, c.Request.Body)
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	key := c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("uploadId", bucketName, c.Request.URL.Path, nil))
		return
	}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	writer, err := factory.GetMultipartUploadWriter(uploadId)
	if err == errno.NoSuchUpload {
		c.XML(http.StatusNotFound, NoSuchUpload(bucketName, c.Request.URL.Path, key))
		return
	}
	parts := writer.ListParts()
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	key := c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("uploadId", bucketName, c.Request.URL.Path, nil))
		return
	}
	partID, err := parsePartID(c.Query("partNumber"))
	if err != nil {
		c.XML(http.StatusBadRequest, InvalidArgument("partNumber", bucketName, c.Request.URL.Path, nil))
		return
	}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	writer, err := factory.GetMultipartUploadWriter(uploadId)
	if err != nil {
		if err == errno.NoSuchUpload {
			c.XML(http.StatusNotFound, NoSuchUpload(bucketName, c.Request.URL.Path, key))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	blockInfo, err := writer.GetPartBlockInfo(partID)
	if err != nil {
		switch err {
		case errno.MethodNotAllowed:
			c.XML(http.StatusMethodNotAllowed, MethodNotAllowed(bucketName, c.Request.URL.Path, &key))
			return
		case errno.InvalidArgument:
			c.XML(http.StatusBadRequest, InvalidArgument("partNumber", bucketName, c.Request.URL.Path, nil))
			return
		case errno.InvalidPart:
			c.XML(http.StatusNotFound, InvalidPart(bucketName, c.Request.URL.Path, key))
			return
		case errno.InvalidObjectState:
			c.XML(http.StatusConflict, InvalidObjectState(bucketName, c.Request.URL.Path, key))
			return
		default:
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
			return
		}
	}
	reader := factory.GetEcosReader(key)
	content, err := reader.GetBlock(blockInfo)
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	if uint64(len(content)) != blockInfo.BlockSize {
		c.XML(http.StatusPreconditionFailed, PreconditionFailed(bucketName, c.Request.URL.Path, "Incompatible Part Size", &key))
		return
	}
	fullReader := bytes.NewReader(content)
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	key := c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("uploadId", bucketName, c.Request.URL.Path, nil))
		return
	}
	var completeRequest CompleteMultipartUpload
	err := xml.NewDecoder(c.Request.Body).Decode(&completeRequest)
	if err != nil {
		c.XML(http.StatusBadRequest, MalformedXML(bucketName, c.Request.URL.Path, &key))
		return
	}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	etag, err := factory.CompleteMultipartUploadJob(uploadId)
	if err != nil {
		switch err {
		case errno.InvalidPartOrder:
			c.XML(http.StatusBadRequest, InvalidPartOrder(bucketName, c.Request.URL.Path, key))
			return
		case errno.NoSuchUpload:
			c.XML(http.StatusNotFound, NoSuchUpload(bucketName, c.Request.URL.Path, key))
			return
		case errno.InvalidPart:
			c.XML(http.StatusNotFound, InvalidPart(bucketName, c.Request.URL.Path, key))
			return
		default:
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
			return
		}
	}
	c.XML(http.StatusOK, CompleteMultipartUploadResult{
		Location: common.PtrString(fmt.Sprintf("%s/%s", bucketName, key)),
		Bucket:   common.PtrString(bucketName),
		Key:      common.PtrString(key),
		ETag:     common.PtrString(etag),
	})
}

// abortMultipartUpload aborts a multipart upload
func abortMultipartUpload(c *gin.Context) {
	bucketName := c.Param("bucketName")
	if bucketName == "" {
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	key := c.Param("key")
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	uploadId := c.Query("uploadId")
	if uploadId == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("uploadId", bucketName, c.Request.URL.Path, nil))
		return
	}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	err = factory.AbortMultipartUploadJob(uploadId)
	if err != nil {
		switch err {
		case errno.NoSuchUpload:
			c.XML(http.StatusNotFound, NoSuchUpload(bucketName, c.Request.URL.Path, key))
			return
		default:
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
			return
		}
	}
	c.Status(http.StatusNoContent)
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	prefix := c.Query("prefix")
	//keyMarker := c.Query("keyMarker")
	//if keyMarker == "" {
	//	c.XML(http.StatusBadRequest, gin.H{"error": errno.MissingKeyMarker.Error()})
	//	return
	//}
	//uploadIdMarker := c.Query("uploadIdMarker")
	//if uploadIdMarker == "" {
	//	c.XML(http.StatusBadRequest, gin.H{"error": errno.MissingUploadIdMarker.Error()})
	//	return
	//}
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
		return
	}
	uploads, err := factory.ListMultipartUploadJob()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
		return
	}
	var uploadsWithPrefix []types.MultipartUpload
	for _, upload := range uploads {
		if prefix != "" {
			prefix = object.CleanObjectKey(prefix)
		}
		if strings.HasPrefix(object.CleanObjectKey(*upload.Key), prefix) {
			uploadsWithPrefix = append(uploadsWithPrefix, upload)
		}
	}
	ret := ListMultipartUploadsResult{
		Bucket:      common.PtrString(bucketName),
		IsTruncated: common.PtrBool(false),
		Uploads:     uploadsWithPrefix,
	}
	delimiter := c.Query("delimiter")
	maxUploads, _ := strconv.Atoi(c.Query("maxUploads"))
	if maxUploads < 0 {
		c.XML(http.StatusBadRequest, InvalidArgument("maxUploads", bucketName, c.Request.URL.Path, nil))
		return
	}
	if maxUploads > 0 {
		ret.MaxUploads = maxUploads
	}
	if delimiter != "" {
		ret.Delimiter = common.PtrString(delimiter)
	}
	if prefix != "" {
		ret.Prefix = common.PtrString(prefix)
	}
	c.XML(http.StatusOK, ret)
}
