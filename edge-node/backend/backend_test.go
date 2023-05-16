package backend

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	"ecos/edge-node/infos"
	gateway "ecos/gateway/router"
	"ecos/shared"
	"ecos/utils/common"
	"ecos/utils/logger"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestBackendManual(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"
	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
	_ = common.InitAndClearPath(basePath)
	watchers, alayas, _ := shared.RunTestEdgeNodeCluster(t, ctx, false, basePath, 5)
	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort
	c, err := client.New(&conf)

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")
	err = c.GetVolumeOperator().CreateBucket(bucketInfo)
	assert.NoError(t, err, "Failed to create default bucket")

	listenOn := "0.0.0.0:3269"

	backend := NewBackend(ctx, listenOn, alayas[0], nil, watchers[0], c)
	backend.Run()

	go func() {
		// Gen Gateway
		logger.Infof("Start init Gateway ...")
		gatewayConf := gateway.DefaultConfig
		gatewayConf.ClientConfig = conf
		g := gateway.NewRouter(gatewayConf)
		_ = g.Run()
	}()

	time.Sleep(time.Second * 10)

}

func TestBackend_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"
	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
	_ = common.InitAndClearPath(basePath)
	watchers, alayas, _ := shared.RunTestEdgeNodeCluster(t, ctx, false, basePath, 5)
	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort
	c, err := client.New(&conf)

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")
	err = c.GetVolumeOperator().CreateBucket(bucketInfo)
	assert.NoError(t, err, "Failed to create default bucket")

	listenOn := "127.0.0.1:3269"
	backend := NewBackend(ctx, listenOn, alayas[0], nil, watchers[0], c)

	time.Sleep(time.Second * 10)

	engine := backend.GetRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/cluster_report", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code, "Failed to get cluster info")
}
