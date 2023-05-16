/**
 * Backend package contains the backend logic of the application.
 * Backend 用于提供 REST 风格的 API 服务，供前端进行查询和交互
 */

package backend

import (
	"ecos/client"
	"ecos/client/io"
	"ecos/edge-node/alaya"
	"ecos/edge-node/gaia"
	"ecos/edge-node/object"
	"ecos/edge-node/watcher"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/context"
	"google.golang.org/protobuf/types/known/emptypb"
	"strings"
	"sync"
)

// use gin as the web framework
type Backend struct {
	ctx         context.Context
	listenOn    string
	nodeAlaya   alaya.Alayaer
	nodeGaia    *gaia.Gaia
	nodeWatcher *watcher.Watcher
	ecosClient  *client.Client

	putTaskMap sync.Map
}

func NewBackend(ctx context.Context, listenOn string,
	nodeAlaya alaya.Alayaer, nodeGaia *gaia.Gaia, nodeWatcher *watcher.Watcher,
	ecosClient *client.Client) *Backend {

	return &Backend{
		ctx:         ctx,
		listenOn:    listenOn,
		nodeAlaya:   nodeAlaya,
		nodeGaia:    nodeGaia,
		nodeWatcher: nodeWatcher,
		ecosClient:  ecosClient,

		putTaskMap: sync.Map{},
	}
}

func (b *Backend) GetRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/cluster_report", b.getClusterReport)
	router.PUT("/object/*object_id", b.PutObject)
	router.GET("/state/task/put/*object_id", b.GetPutObjectState)
	router.GET("/bucket/*bucketID", b.ListObject)
	return router
}

func (b *Backend) Run() {
	router := b.GetRouter()
	router.Run(b.listenOn)
}

func (b *Backend) ListObject(c *gin.Context) {
	objects, err := b.ecosClient.ListObjects(b.ctx, "default", "")
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, objects)
}

func (b *Backend) PutObject(c *gin.Context) {
	objectId := c.Param("object_id")
	if objectId == "" {
		c.JSON(400, gin.H{
			"error": "object_id is empty",
		})
		return
	}

	_, bucketID, key, err := object.SplitPrefixWithoutSlotID(objectId)
	if err != nil {
		c.JSON(400, gin.H{
			"error": err.Error(),
		})
		return
	}
	// split bucketID : /volumeID/bucketName
	bucketID = strings.Trim(bucketID, "/")
	split := strings.SplitN(bucketID, "/", 2)
	bucketName := split[1]
	ecosIO, err := b.ecosClient.GetIOFactory(bucketName)
	if (ecosIO == nil) || (err != nil) {
		c.JSON(500, gin.H{
			"error": "failed to get IOFactory",
		})
		return
	}
	writer := ecosIO.GetEcosWriter(key)
	if writer == nil {
		c.JSON(500, gin.H{
			"error": "failed to get writer",
		})
		return
	}

	b.putTaskMap.Store(objectId, writer)

	buf := make([]byte, 1024)
	for {
		n, err := c.Request.Body.Read(buf)
		if err != nil {
			break
		}
		n, err = writer.Write(buf[:n])
		if err != nil {
			break
		}
	}
	_ = writer.CloseAsync()
	c.JSON(200, gin.H{
		"error": "",
	})
}

func (b *Backend) GetPutObjectState(c *gin.Context) {
	objectId := c.Param("object_id")
	if objectId == "" {
		c.JSON(400, gin.H{
			"error": "object_id is empty",
		})
	}
	writer, ok := b.putTaskMap.Load(objectId)
	if !ok {
		c.JSON(400, gin.H{
			"error": "object_id not found",
		})
	}
	state := writer.(*io.EcosWriter).GetState()
	// json -> str
	data, err := json.Marshal(state)
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
	}
	c.Data(200, "application/json", data)
}

func (b *Backend) getClusterReport(c *gin.Context) {
	m := b.nodeWatcher.Monitor
	rpt, err := m.GetClusterReport(b.ctx, &emptypb.Empty{})
	if err != nil {
		c.JSON(500, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(200, rpt)
}
