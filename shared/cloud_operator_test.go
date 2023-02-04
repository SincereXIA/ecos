package shared

import (
	"context"
	client2 "ecos/client"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

var cloudClientConf = config.DefaultConfig
var edgeClientConf = config.DefaultConfig

var cloudClient *client2.Client
var edgeClient *client2.Client

func putObject(t *testing.T, ctx context.Context, objName string, data []byte, connectType int) {
	var client *client2.Client
	switch connectType {
	case config.ConnectCloud:
		client = cloudClient
	case config.ConnectEdge:
		client = edgeClient
	}
	factory, err := client.GetIOFactory("default")
	assert.NoError(t, err)
	writer := factory.GetEcosWriter(objName)
	_, err = writer.Write(data)
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)

	//_, err := client.PutObject(ctx, objName, bytes.NewReader(data))
	assert.NoError(t, err)
}

func init() {
	cloudClientConf.ConnectType = config.ConnectCloud
	edgeClientConf.ConnectType = config.ConnectEdge
}

func TestNewCloudBucketOperator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	basePath := "./ecos-data/"

	t.Cleanup(func() {
		_ = os.RemoveAll(basePath)
	})

	watchers, _, _ := RunTestEdgeNodeCluster(t, ctx, false, basePath, 5)
	time.Sleep(3 * time.Second)

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort
	cloudAddr := strings.Split(watchers[0].Config.SunAddr, ":")[0]
	cloudPort, _ := strconv.Atoi(strings.Split(watchers[0].Config.SunAddr, ":")[1])
	conf.CloudAddr = cloudAddr
	conf.CloudPort = uint64(cloudPort)
	conf.ConnectType = config.ConnectCloud

	edgeConf := conf
	edgeConf.ConnectType = config.ConnectEdge

	client, err := client2.New(&conf)
	edgeClient, err := client2.New(&edgeConf)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
	}
	edgeBucketInfo := infos.GenBucketInfo("root", "edge", "root")
	edgeClient.GetVolumeOperator().CreateBucket(edgeBucketInfo)
	objectNum := 20
	objectSize := 1024 * 1024 * 10 //10M
	factory, err := client.GetIOFactory("default")
	edgeFactory, err := edgeClient.GetIOFactory("edge")

	e2cFactory, err := edgeClient.GetIOFactory("default")
	c2eFactory, err := client.GetIOFactory("edge")

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

			writer = edgeFactory.GetEcosWriter("/test_edge_" + strconv.Itoa(i) + "/ecos-test")
			size, err = writer.Write(data)
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

		bucket, err = cloudVolumeOperator.Get("edge")
		objects, err = bucket.List("/")
		assert.NoError(t, err, "Failed to list objects")
		assert.Equal(t, objectNum, len(objects), "object num not match")

		edgeVolumeOperator := edgeClient.GetVolumeOperator()
		bucket, err = edgeVolumeOperator.Get("edge")
		objects, err = bucket.List("/")
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

		reader = e2cFactory.GetEcosReader("/test_0/ecos-test")
		data = make([]byte, objectSize)
		size, err = reader.Read(data)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to read data: %v", err)
		}
		assert.Equal(t, objectSize, size, "data size not match")

		reader = e2cFactory.GetEcosReader("/test_0/ecos-test")
		data = make([]byte, objectSize)
		size, err = reader.Read(data)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to read data: %v", err)
		}
		assert.Equal(t, objectSize, size, "data size not match")

		reader = c2eFactory.GetEcosReader("/test_edge_0/ecos-test")
		data = make([]byte, objectSize)
		size, err = reader.Read(data)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to read data: %v", err)
		}
		assert.Equal(t, objectSize, size, "data size not match")
	})

	t.Run("test object change", func(t *testing.T) {
		writer := edgeFactory.GetEcosWriter("edgeData")
		data := "first data"
		_, err := writer.Write([]byte(data))
		if err != nil {
			t.Errorf("Failed to write data: %v", err)
		}
		err = writer.Close()
		assert.NoError(t, err, "Failed to write data")

		reader := c2eFactory.GetEcosReader("edgeData")
		buf := make([]byte, len(data))
		reader.Read(buf)
		assert.Equal(t, data, string(buf), "data not match")

		writer = edgeFactory.GetEcosWriter("edgeData")
		data = "second data"
		_, err = writer.Write([]byte(data))
		if err != nil {
			t.Errorf("Failed to write data: %v", err)
		}
		err = writer.Close()
		assert.NoError(t, err, "Failed to write data")

		reader = c2eFactory.GetEcosReader("edgeData")
		buf = make([]byte, len(data))
		reader.Read(buf)
		assert.Equal(t, data, string(buf), "data not match")
	})

}
