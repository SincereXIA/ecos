package shared

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"ecos/utils/common"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestCluster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"

	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
	_ = common.InitAndClearPath(basePath)
	watchers, _, _ := RunTestEdgeNodeCluster(t, ctx, false, basePath, 9)
	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort
	c, err := client.New(&conf)

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")
	err = c.GetVolumeOperator().CreateBucket(bucketInfo)
	assert.NoError(t, err, "Failed to create default bucket")

	time.Sleep(time.Minute * 5)
}
