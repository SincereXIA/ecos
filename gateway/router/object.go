package router

import (
	"bufio"
	"ecos/edge-node/object"
	"ecos/edge-node/watcher"
	"ecos/utils/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"encoding/xml"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// putObject creates a new object
func putObject(c *gin.Context) {
	bucketName, key, err := parseBucketKey(c)
	if err != nil {
		return
	}
	body := c.Request.Body
	factory, err := getFactory(c, bucketName, key)
	if err != nil {
		return
	}
	writer := factory.GetEcosWriter(key)
	// See https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-streaming.html
	switch c.GetHeader("X-Amz-Content-Sha256") {
	case "STREAMING-AWS4-HMAC-SHA256-PAYLOAD":
		bufBody := bufio.NewReader(body)
		for {
			line, err := bufBody.ReadString('\n')
			logger.Tracef("Got Signature Line: %s", line)
			if err != nil {
				if err == io.EOF {
					break
				}
				c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
				return
			}
			length, _, err := parseSignature(line)
			if err != nil {
				if err == errno.SignatureDoesNotMatch {
					c.XML(http.StatusBadRequest, InvalidArgument("signature", bucketName, c.Request.URL.Path, &key))
					return
				}
				c.XML(http.StatusBadRequest, SignatureDoesNotMatch(bucketName, c.Request.URL.Path, &key))
				return
			}
			n, err := io.CopyN(writer, bufBody, length)
			if n != length {
				c.XML(http.StatusPreconditionFailed, PreconditionFailed(bucketName, c.Request.URL.Path,
					errno.IncompatibleSize.Error()+" with err "+err.Error(), &key))
				return
			}
			line, err = bufBody.ReadString('\n')
			if line != "\r\n" || err != nil {
				c.XML(http.StatusBadRequest, IncompleteBody(bucketName, c.Request.URL.Path, key))
			}
		}
	case "STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER":
		logger.Warningf("not implemented")
		fallthrough
	case "", "UNSIGNED-PAYLOAD":
		fallthrough
	default:
		if _, err := io.Copy(writer, body); err != nil {
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
			return
		}
	}
	err = writer.Close()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	c.Status(http.StatusOK)
}

// postObject creates a new object by post form
func postObject(c *gin.Context) {
	bucketName, err := parseBucket(c)
	if err != nil {
		return
	}
	mForm, err := c.MultipartForm()
	if err != nil {
		c.XML(http.StatusBadRequest, MalformedPOSTRequest(bucketName, c.Request.URL.Path, nil))
		return
	}
	if mForm.Value == nil {
		c.XML(http.StatusBadRequest, RequestIsNotMultiPartContent(bucketName, c.Request.URL.Path))
		return
	}
	if len(mForm.Value["key"]) != 1 {
		c.XML(http.StatusBadRequest, IncorrectNumberOfFilesInPostRequest(bucketName, c.Request.URL.Path, nil))
		return
	}
	key := mForm.Value["key"][0]
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	if len(mForm.File["file"]) != 1 {
		c.XML(http.StatusBadRequest, IncorrectNumberOfFilesInPostRequest(bucketName, c.Request.URL.Path, nil))
		return
	}
	file := mForm.File["file"][0]
	if file == nil {
		c.XML(http.StatusBadRequest, InvalidArgument("file", bucketName, c.Request.URL.Path, nil))
		return
	}
	content, err := file.Open()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	factory, err := getFactory(c, bucketName, key)
	if err != nil {
		return
	}
	writer := factory.GetEcosWriter(key)
	_, err = io.Copy(writer, content)
	if err != nil && err != io.EOF {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	err = writer.Close()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	c.Status(http.StatusOK)
}

// headObject gets an object meta
func headObject(c *gin.Context) {
	bucketName, key, err := parseBucketKey(c)
	if err != nil {
		return
	}
	// Get Object Operator
	op, err := getObjectOperator(c, bucketName, key)
	if err != nil {
		return
	}
	info, err := op.Info()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	meta := info.(*object.ObjectMeta)
	c.Header("Content-Length", strconv.FormatUint(meta.ObjSize, 10))
	c.Header("ETag", meta.ObjHash)
	c.Header("Last-Modified", meta.UpdateTime.Format(http.TimeFormat))
	c.Status(http.StatusOK)
}

// getObject gets an object
func getObject(c *gin.Context) {
	bucketName, key, err := parseBucketKey(c)
	if err != nil {
		return
	}
	// Get Object Operator
	op, err := getObjectOperator(c, bucketName, key)
	if err != nil {
		return
	}
	info, err := op.Info()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	meta := info.(*object.ObjectMeta)
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	reader := factory.GetEcosReader(key)
	c.Header("ETag", meta.ObjId)
	requestRange := c.Request.Header.Get("Range")
	if requestRange == "" {
		c.DataFromReader(http.StatusOK, int64(meta.ObjSize), "application/octet-stream", reader, map[string]string{
			"Last-Modified": meta.UpdateTime.Format(http.TimeFormat),
		})
		return
	}
	ranges, err := parseRange(requestRange, int64(meta.ObjSize))
	if err != nil {
		c.XML(http.StatusRequestedRangeNotSatisfiable, InvalidRange(bucketName, c.Request.URL.Path, key))
		return
	}
	if len(ranges) == 0 {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if len(ranges) == 1 {
		_, err = reader.Seek(ranges[0].start, io.SeekStart)
		if err != nil {
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
			return
		}
		c.Status(http.StatusPartialContent)
		c.Header("Last-Modified", meta.UpdateTime.Format(http.TimeFormat))
		c.Header("Content-Range", ranges[0].contentRange(int64(meta.ObjSize)))
		_, err = io.CopyN(c.Writer, reader, ranges[0].length)
		if err != nil {
			c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
			return
		}
		return
	}
	c.Status(http.StatusPartialContent)
	c.Header("Last-Modified", meta.UpdateTime.Format(http.TimeFormat))
	if len(ranges) > 1 {
		multipartWriter := multipart.NewWriter(c.Writer)
		c.Header("Content-Type", "multipart/byteranges; boundary="+multipartWriter.Boundary())
		c.Header("ETag", meta.ObjId)
		for _, r := range ranges {
			_, err = reader.Seek(ranges[0].start, io.SeekStart)
			if err != nil {
				c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
				return
			}
			header := map[string][]string{
				"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, meta.ObjSize)},
			}
			partWriter, err := multipartWriter.CreatePart(header)
			if err != nil {
				c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
				return
			}
			_, err = reader.Seek(r.start, io.SeekStart)
			if err != nil && err != io.EOF {
				c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
				return
			}
			_, err = io.CopyN(partWriter, reader, r.length)
			if err != nil && err != io.EOF {
				c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
				return
			}
		}
	}
}

// deleteObject deletes an object
func deleteObject(c *gin.Context) {
	bucketName, key, err := parseBucketKey(c)
	if err != nil {
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	// Delete Object from Bucket Operator
	err = op.Remove(key)
	if err != nil && !strings.Contains(err.Error(), errno.MetaNotExist.Error()) {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	c.Status(http.StatusNoContent)
}

type CopyObjectResult struct {
	ETag *string `xml:"ETag"`
}

func parseSrcKey(src string) (bucket, key string, err error) {
	src = strings.Trim(src, "/")
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
	bucketName, key, err := parseBucketKey(c)
	if err != nil {
		return
	}
	src := c.GetHeader("x-amz-copy-source")
	if src == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("x-amz-copy-source", bucketName, c.Request.URL.Path, nil))
		return
	}
	srcBucket, srcKey, err := parseSrcKey(src)
	if err != nil {
		c.XML(http.StatusBadRequest, InvalidArgument("x-amz-copy-source", bucketName, c.Request.URL.Path, nil))
		return
	}
	if srcBucket == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("x-amz-copy-source", bucketName, c.Request.URL.Path, nil))
		return
	}
	if key == "" {
		c.XML(http.StatusBadRequest, InvalidArgument("key", bucketName, c.Request.URL.Path, nil))
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(srcBucket)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(srcBucket))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	// Get Object Operator
	op, err = op.Get(srcKey)
	if err != nil {
		if strings.Contains(err.Error(), errno.MetaNotExist.Error()) {
			c.XML(http.StatusNotFound, NoSuchKey(srcBucket, c.Request.URL.Path, srcKey))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	info, err := op.Info()
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	meta := info.(*object.ObjectMeta)
	factory, err := Client.GetIOFactory(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
	writer := factory.GetEcosWriter(key)
	etag, err := writer.Copy(meta)
	if err != nil {
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, &key))
		return
	}
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
		c.XML(http.StatusBadRequest, InvalidBucketName(nil))
		return
	}
	body := c.Request.Body
	var deleteRequest Delete
	err := xml.NewDecoder(body).Decode(&deleteRequest)
	if err != nil {
		c.XML(http.StatusBadRequest, MalformedXML(bucketName, c.Request.URL.Path, nil))
		return
	}
	// Get Bucket Operator
	op, err := Client.GetVolumeOperator().Get(bucketName)
	if err != nil {
		if strings.Contains(err.Error(), errno.InfoNotFound.Error()) {
			c.XML(http.StatusNotFound, NoSuchBucket(bucketName))
			return
		}
		c.XML(http.StatusInternalServerError, InternalError(err.Error(), bucketName, c.Request.URL.Path, nil))
		return
	}
	var deleteResult DeleteResult
	for _, obj := range deleteRequest.Object {
		// Delete Objects from Bucket Operator
		logger.Infof("Delete Object: %s", *obj.Key)
		err = op.Remove(*obj.Key)
		if err != nil && !strings.Contains(err.Error(), errno.MetaNotExist.Error()) {
			deleteResult.Error = append(deleteResult.Error, DeleteError{
				Code:    common.PtrString("InternalError"),
				Key:     obj.Key,
				Message: common.PtrString(err.Error()),
			})
			logger.Warningf("Delete Object Failed: %s", *obj.Key)
		} else {
			deleteResult.Deleted = append(deleteResult.Deleted, DeletedObject(obj))
			logger.Infof("Delete Object Success: %s", *obj.Key)
		}
	}
	c.XML(http.StatusOK, deleteResult)
}

// objectLevelPostHandler handles object level POST requests
// POST /{bucketName}/{key} Include:
//  CreateMultiPartUpload:   POST /Key+?uploads
//  CompleteMultipartUpload: POST /Key+?uploadId=UploadId
func objectLevelPostHandler(c *gin.Context) {
	if c.Param("key") == "/" {
		bucketLevelPostHandler(c)
		return
	}
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
	if c.Param("key") == "/" {
		createBucket(c)
		return
	}
	if _, ok := c.GetQuery("uploadId"); ok {
		uploadPart(c)
		defer func() {
			if metricChan, ok := c.Get("metric"); ok {
				*metricChan.(*chan string) <- watcher.MetricsGatewayPartPutTimer
			}
		}()
		return
	}
	if c.GetHeader("x-amz-copy-source") != "" {
		copyObject(c)
		return
	}
	putObject(c)
	defer func() {
		if metricChan, ok := c.Get("metric"); ok {
			*metricChan.(*chan string) <- watcher.MetricsGatewayPutTimer
		}
	}()
}

// objectLevelGetHandler handles object level GET requests
//
// GET /{bucketName}/{key} Include:
//  GetObject: GET /Key+?partNumber=PartNumber&response-cache-control=ResponseCacheControl&response-content-disposition=ResponseContentDisposition&response-content-encoding=ResponseContentEncoding&response-content-language=ResponseContentLanguage&response-content-type=ResponseContentType&response-expires=ResponseExpires&versionId=VersionId
//  ListParts: GET /Key+?max-parts=MaxParts&part-number-marker=PartNumberMarker&uploadId=UploadId
func objectLevelGetHandler(c *gin.Context) {
	if c.Param("key") == "/" {
		bucketLevelGetHandler(c)
		return
	}
	if _, ok := c.GetQuery("uploadId"); ok {
		listParts(c)
		return
	}
	if _, ok := c.GetQuery("partNumber"); ok {
		getPart(c)
		defer func() {
			if metricChan, ok := c.Get("metric"); ok {
				*metricChan.(*chan string) <- watcher.MetricsGatewayPartGetTimer
			}
		}()
		return
	}
	getObject(c)
	defer func() {
		if metricChan, ok := c.Get("metric"); ok {
			*metricChan.(*chan string) <- watcher.MetricsGatewayGetTimer
		}
	}()
}

// objectLevelDeleteHandler handles object level DELETE requests
//
// DELETE /{bucketName}/{key} Include:
//  DeleteObject:         DELETE /Key+?versionId=VersionId
//  AbortMultipartUpload: DELETE /Key+?uploadId=UploadId
func objectLevelDeleteHandler(c *gin.Context) {
	if c.Param("key") == "/" {
		bucketLevelDeleteHandler(c)
		return
	}
	if _, ok := c.GetQuery("uploadId"); ok {
		abortMultipartUpload(c)
		return
	}
	deleteObject(c)
	defer func() {
		if metricChan, ok := c.Get("metric"); ok {
			*metricChan.(*chan string) <- watcher.MetricsGatewayDeleteTimer
		}
	}()
}

// objectLevelHeadHandler handles object level HEAD requests
//
// HEAD /{bucketName}/{key} Include:
//  HeadObject: HEAD /Key+?versionId=VersionId
func objectLevelHeadHandler(c *gin.Context) {
	if c.Param("key") == "/" {
		bucketLevelHeadHandler(c)
		return
	}
	headObject(c)
	defer func() {
		if metricChan, ok := c.Get("metric"); ok {
			*metricChan.(*chan string) <- watcher.MetricsGatewayStatTimer
		}
	}()
}
