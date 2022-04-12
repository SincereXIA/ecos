package client

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/infos"
	edgeNodeTest "ecos/edge-node/test"
	"ecos/utils/common"
	"ecos/utils/logger"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"
	objectNum := 20

	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
	_ = common.InitAndClearPath(basePath)
	watchers, _ := edgeNodeTest.RunTestEdgeNodeCluster(t, ctx, true, basePath, 9)

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort

	client, err := New(&conf)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
	}

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")
	err = client.GetVolumeOperator().CreateBucket(bucketInfo)
	assert.NoError(t, err, "Failed to create default bucket")

	factory := client.GetIOFactory(bucketName)
	objectSize := 1024 * 1024 * 10 //10M
	for i := 0; i < objectNum; i++ {
		data := genTestData(objectSize)
		writer := factory.GetEcosWriter("test" + strconv.Itoa(i))
		size, err := writer.Write(data)
		err = writer.Close()
		assert.NoError(t, err, "Failed to write data")
		assert.Equal(t, objectSize, size, "data size not match")
	}

	t.Run("test get object meta", func(t *testing.T) {
		meta, err := client.ListObjects(ctx, bucketName)
		assert.NoError(t, err, "Failed to list objects")
		assert.Equal(t, objectNum, len(meta), "object count not match")
	})

	t.Run("delete object", func(t *testing.T) {
		operator := client.GetVolumeOperator()
		bucket, err := operator.Get("default")
		assert.NoError(t, err, "Failed to get bucket")
		err = bucket.Remove("test0")
		assert.NoError(t, err, "Failed to remove object")
		_, err = bucket.Get("test0")
		assert.Error(t, err, "get removed object should fail")
		time.Sleep(time.Second)
	})
}

func genTestData(size int) []byte {
	rand.Seed(time.Now().Unix())
	directSize := 1024 * 1024 * 10
	if size < directSize {
		data := make([]byte, size)
		for idx := range data {
			if idx%100 == 0 {
				data[idx] = '\n'
			} else {
				data[idx] = byte(rand.Intn(26) + 97)
			}
		}
		return data
	}
	d := make([]byte, directSize)
	data := make([]byte, 0, size)
	rand.Read(d)
	for size-directSize > 0 {
		data = append(data, d...)
		size = size - directSize
	}
	data = append(data, d[0:size]...)
	return data
}

func BenchmarkClient(b *testing.B) {
	logger.Logger.SetLevel(logrus.FatalLevel)
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"
	b.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
	_ = common.InitAndClearPath(basePath)
	watchers, _ := edgeNodeTest.RunTestEdgeNodeCluster(b, ctx, true, basePath, 9)

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort

	client, err := New(&conf)
	if err != nil {
		b.Errorf("Failed to create client: %v", err)
	}

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")
	err = client.GetVolumeOperator().CreateBucket(bucketInfo)
	if err != nil {
		b.Errorf("Failed to create default bucket: %v", err)
	}

	factory := client.GetIOFactory(bucketName)
	sizeMap := map[string]int{
		"10K":  1024 * 10,
		"1M":   1024 * 1024,
		"10M":  1024 * 1024 * 10,
		"100M": 1024 * 1024 * 100,
	}

	b.Run("put object", func(b *testing.B) {
		for key, objectSize := range sizeMap {
			b.Run(key, func(b *testing.B) {
				//gen test data
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					b.StopTimer()
					data := genTestData(objectSize)
					b.StartTimer()
					writer := factory.GetEcosWriter("test" + strconv.Itoa(n))
					size, err := writer.Write(data)
					err = writer.Close()
					if err != nil {
						b.Errorf("Failed to write data: %v", err)
					}
					if size != objectSize {
						b.Errorf("data size not match: %d", size)
					}
				}
				b.StopTimer()
			})
		}
	})

	b.Run("get object", func(b *testing.B) {
		for key, objectSize := range sizeMap {
			b.Run(key, func(b *testing.B) {
				data := genTestData(objectSize)
				writer := factory.GetEcosWriter("test" + strconv.Itoa(0))
				_, _ = writer.Write(data)
				err = writer.Close()
				if err != nil {
					b.Errorf("Failed to write data: %v", err)
				}
				b.ResetTimer()
				b.StartTimer()
				for n := 0; n < b.N; n++ {
					reader := factory.GetEcosReader("test0")
					data := make([]byte, objectSize)
					_, err := reader.Read(data)
					//data, err := ioutil.ReadAll(reader)
					if err != nil && err != io.EOF {
						b.Errorf("Failed to read data: %v", err)
					}
					if len(data) != objectSize {
						b.Errorf("data size not match: %d", len(data))
					}
				}
				b.StopTimer()
			})
		}
	})
}
