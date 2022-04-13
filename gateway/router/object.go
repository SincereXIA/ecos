package router

import (
	"ecos/edge-node/object"
	"ecos/utils/errno"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
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
	//if c.Request.Body == nil || c.Request.ContentLength == 0 {
	//	c.JSON(http.StatusBadRequest, gin.H{"error": "Request body is missing"})
	//	return
	//}
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

type ObjectIdentifier struct {
	Key string `xml:"Key"`
	// VersionId string `xml:"VersionId"`
}

type DeletedObject struct {
	// DeleteMarker string `xml:"DeleterMarker"`
	// DeleteMarkerVersion string `xml:"DeleterMarkerVersion"`
	Key string `xml:"Key"`
	// VersionId string `xml:"VersionId"`
}

type DeleteError struct {
	Code    string `xml:"Code"`
	Key     string `xml:"Key"`
	Message string `xml:"Message"`
	// VersionId string `xml:"VersionId"`
}

type Delete struct {
	Object []ObjectIdentifier `xml:"Object"`
	Quiet  bool               `xml:"Quiet"`
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
	for _, obj := range deleteRequest.Object {
		// Delete Objects from Bucket Operator
		err = op.Remove(obj.Key)
		if err != nil {
			deleteResult.Error = append(deleteResult.Error, DeleteError{
				Code:    "InternalError",
				Key:     obj.Key,
				Message: err.Error(),
			})
		} else {
			deleteResult.Deleted = append(deleteResult.Deleted, DeletedObject(obj))
		}
	}
	c.Header("Server", "Ecos")
	c.XML(http.StatusOK, deleteResult)
}
