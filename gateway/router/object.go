package router

import (
	"ecos/edge-node/object"
	"ecos/edge-node/watcher"
	"ecos/utils/errno"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/rcrowley/go-metrics"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	factory := Client.GetIOFactory(bucketName)
	if factory == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.BucketNotFound.Error()})
		return
	}
	writer := factory.GetEcosWriter(key)
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
	factory := Client.GetIOFactory(bucketName)
	if factory == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.BucketNotFound.Error()})
		return
	}
	writer := factory.GetEcosWriter(key)
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
	// Get Object Operator
	op, err = op.Get(key)
	if err != nil {
		if strings.Contains(err.Error(), errno.MetaNotExist.Error()) {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    http.StatusNotFound,
				"message": errno.ObjectNotFound.Error(),
			})
			return
		}
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
	c.Header("Last-Modified", meta.UpdateTime.Format(http.TimeFormat))
	c.Header("Server", "Ecos")
	c.Status(http.StatusOK)
}

// Codes Below from golang.org/x/net/http/fs.go
// License: github.com/golang/go/LICENSE

/*
Copyright (c) 2009 The Go Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

type httpRange struct {
	start, length int64
}

func (r httpRange) contentRange(size int64) string {
	return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size)
}

// parseRange parses a Range header string as per RFC 7233.
// errNoOverlap is returned if none of the ranges overlap.
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" {
		return nil, nil // header not present
	}
	const b = "bytes="
	if !strings.HasPrefix(s, b) {
		return nil, errors.New("invalid range")
	}
	var ranges []httpRange
	noOverlap := false
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = textproto.TrimString(ra)
		if ra == "" {
			continue
		}
		i := strings.Index(ra, "-")
		if i < 0 {
			return nil, errors.New("invalid range")
		}
		start, end := textproto.TrimString(ra[:i]), textproto.TrimString(ra[i+1:])
		var r httpRange
		if start == "" {
			// If no start is specified, end specifies the
			// range start relative to the end of the file.
			i, err := strconv.ParseInt(end, 10, 64)
			if err != nil {
				return nil, errors.New("invalid range")
			}
			if i > size {
				i = size
			}
			r.start = size - i
			r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 {
				return nil, errors.New("invalid range")
			}
			if i >= size {
				// If the range begins after the size of the content,
				// then it does not overlap.
				noOverlap = true
				continue
			}
			r.start = i
			if end == "" {
				// If no end is specified, range extends to end of the file.
				r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i {
					return nil, errors.New("invalid range")
				}
				if i >= size {
					i = size - 1
				}
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	if noOverlap && len(ranges) == 0 {
		// The specified ranges did not overlap with the content.
		return nil, errors.New("invalid range")
	}
	return ranges, nil
}

// Source codes from golang.org/x/net/http/fs.go ends here
// Codes Above from golang.org/x/net/http/fs.go

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
	c.Header("ETag", meta.ObjId)
	requestRange := c.Request.Header.Get("Range")
	if requestRange == "" {
		c.DataFromReader(http.StatusOK, int64(meta.ObjSize), "application/octet-stream", reader, nil)
		return
	}
	ranges, err := parseRange(requestRange, int64(meta.ObjSize))
	if err != nil {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	c.Header("Accept-Ranges", "bytes")
	if len(ranges) == 0 {
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if len(ranges) == 1 {
		_, err = reader.Seek(ranges[0].start, io.SeekStart)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		c.Status(http.StatusPartialContent)
		_, err = io.CopyN(c.Writer, reader, ranges[0].length)
		if err != nil {
			c.Abort()
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
		}
		return
	}
	c.Status(http.StatusPartialContent)
	if len(ranges) > 1 {
		multipartWriter := multipart.NewWriter(c.Writer)
		for _, r := range ranges {
			_, err = reader.Seek(ranges[0].start, io.SeekStart)
			if err != nil {
				c.Abort()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			header := map[string][]string{
				"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, meta.ObjSize)},
			}
			partWriter, err := multipartWriter.CreatePart(header)
			if err != nil {
				c.Abort()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			_, err = reader.Seek(r.start, io.SeekStart)
			if err != nil && err != io.EOF {
				c.Abort()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
			_, err = io.CopyN(partWriter, reader, r.length)
			if err != nil && err != io.EOF {
				c.Abort()
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}
		}
		c.Status(http.StatusPartialContent)
		c.Header("Content-Type", "multipart/byteranges; boundary="+multipartWriter.Boundary())
		c.Header("ETag", meta.ObjId)
	}
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
	factory := Client.GetIOFactory(bucketName)
	if factory == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errno.BucketNotFound.Error()})
		return
	}
	writer := factory.GetEcosWriter(key)
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
	timer := time.Now()
	if _, ok := c.GetQuery("uploadId"); ok {
		uploadPart(c)
		defer metrics.GetOrRegisterTimer(watcher.MetricsGatewayPartPutTimer, nil).UpdateSince(timer)
		return
	}
	if c.GetHeader("x-amz-copy-source") != "" {
		copyObject(c)
		return
	}
	putObject(c)
	defer metrics.GetOrRegisterTimer(watcher.MetricsGatewayPutTimer, nil).UpdateSince(timer)
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
	timer := time.Now()
	if _, ok := c.GetQuery("uploadId"); ok {
		listParts(c)
		return
	}
	if _, ok := c.GetQuery("partNumber"); ok {
		getPart(c)
		defer metrics.GetOrRegisterTimer(watcher.MetricsGatewayPartGetTimer, nil).UpdateSince(timer)
		return
	}
	getObject(c)
	defer metrics.GetOrRegisterTimer(watcher.MetricsGatewayGetTimer, nil).UpdateSince(timer)
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
}
