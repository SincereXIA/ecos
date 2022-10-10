package common

import (
	client2 "ecos/client"
	"ecos/client/config"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)
import "context"

func TestNewCloudBucketOperator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	basePath := "./ecos-data/"

	t.Cleanup(func() {
		_ = os.RemoveAll(basePath)
	})

	watchers, _, _ := RunTestEdgeNodeCluster(t, ctx, false, basePath, 5)

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort
	cloudAddr := strings.Split(watchers[0].Config.SunAddr, ":")[0]
	cloudPort, _ := strconv.Atoi(strings.Split(watchers[0].Config.SunAddr, ":")[1])
	conf.CloudAddr = cloudAddr
	conf.CloudPort = uint64(cloudPort)
	conf.ConnectType = config.ConnectCloud

	client, err := client2.New(&conf)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
	}
	objectNum := 20
	objectSize := 1024 * 1024 * 10 //10M
	factory, err := client.GetIOFactory("default")
	t.Run("put object", func(t *testing.T) {
		for i := 0; i < objectNum; i++ {
			data := genTestData(objectSize)
			writer := factory.GetEcosWriter("/test_" + strconv.Itoa(i) + "/ecos-test")
			size, err := writer.Write(data)
			if err != nil {
				t.Errorf("Failed to write data: %v", err)
			}
			err = writer.Close()
			assert.NoError(t, err, "Failed to write data")
			assert.Equal(t, objectSize, size, "data size not match")
		}
	})

	t.Run("test list object by cloud", func(t *testing.T) {
		cloudVolumeOperator := client.GetVolumeOperator()
		bucket, err := cloudVolumeOperator.Get("default")
		objects, err := bucket.List("/")
		for _, object := range objects {
			state, _ := object.State()
			t.Logf("object: %v", state)
		}
		assert.NoError(t, err, "Failed to list objects")
		assert.Equal(t, objectNum, len(objects), "object num not match")
	})

	t.Run("get object by cloud", func(t *testing.T) {
		reader := factory.GetEcosReader("/test_0/ecos-test")
		data := make([]byte, objectSize)
		size, err := reader.Read(data)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to read data: %v", err)
		}
		assert.Equal(t, objectSize, size, "data size not match")
	})

}
