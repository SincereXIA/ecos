package gateway

import (
	clientConfig "ecos/client/config"
	clientObject "ecos/client/object"
	"ecos/utils/config"
	"github.com/gin-gonic/gin"
	"io"
)

var ecosIOFactory = clientObject.NewEcosIOFactory(&clientConfig.ClientConfig{
	Config:        config.Config{},
	Object:        clientConfig.DefaultConfig.Object,
	UploadTimeout: 0,
	UploadBuffer:  clientConfig.DefaultConfig.UploadBuffer,
	NodeAddr:      "localhost",
	NodePort:      3267,
})

// putObject creates a new object
func putObject(c *gin.Context) {
	key := c.Param("key")
	body, err := c.Request.GetBody()
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	writer := ecosIOFactory.GetEcosWriter(key)
	_, err = io.Copy(&writer, body)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Status(200)
}

// postObject creates a new object by post form
func postObject(c *gin.Context) {
	mForm, err := c.MultipartForm()
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	key := mForm.Value["key"][0]
	if key == "" {
		c.JSON(400, gin.H{
			"error": "key is empty",
		})
		return
	}
	file := mForm.File["file"][0]
	if file == nil {
		c.JSON(400, gin.H{
			"error": "file is empty",
		})
		return
	}
	content, err := file.Open()
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	writer := ecosIOFactory.GetEcosWriter(key)
	_, err = io.Copy(&writer, content)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.Status(200)
}
