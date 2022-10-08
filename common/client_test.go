package common

import (
	"context"
	client2 "ecos/client"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"ecos/edge-node/watcher"
	"ecos/utils/common"
	"ecos/utils/logger"
	"github.com/elliotchance/orderedmap"
	"github.com/rcrowley/go-metrics"
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
	watchers, alayas, rpcServers := RunTestEdgeNodeCluster(t, ctx, false, basePath, 9)

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort

	client, err := client2.New(&conf)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
	}

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")
	err = client.GetVolumeOperator().CreateBucket(bucketInfo)
	assert.NoError(t, err, "Failed to create default bucket")

	factory, err := client.GetIOFactory(bucketName)
	if err != nil {
		t.Errorf("Failed to get io factory: %v", err)
	}
	objectSize := 1024 * 1024 * 10 //10M
	for i := 0; i < objectNum; i++ {
		data := genTestData(objectSize)
		writer := factory.GetEcosWriter("/test1" + strconv.Itoa(i) + "/ecos-test")
		size, err := writer.Write(data)
		if err != nil {
			t.Errorf("Failed to write data: %v", err)
		}
		err = writer.Close()
		assert.NoError(t, err, "Failed to write data")
		assert.Equal(t, objectSize, size, "data size not match")
	}

	t.Run("test term change", func(t *testing.T) {
		t.Logf("test term change")
		oldTerm := watchers[0].GetCurrentTerm()
		watchers[8].Monitor.Stop()
		watchers[8].GetMoon().Stop()
		alayas[8].Stop()
		rpcServers[8].GracefulStop()
		//watcher.WaitAllTestWatcherOK(watchers[0:8])
		for oldTerm == watchers[0].GetCurrentTerm() {
			logger.Infof("Waiting for term change, now term is %d, oldTerm: %v", watchers[0].GetCurrentTerm(), oldTerm)
			time.Sleep(time.Second)
		}
		WaiteAllAlayaOK(alayas[0:7])
	})

	t.Run("put object", func(t *testing.T) {
		time.Sleep(time.Second * 10)
		for i := 0; i < objectNum; i++ {
			data := genTestData(objectSize)
			writer := factory.GetEcosWriter("/test2" + strconv.Itoa(i) + "/ecos-test")
			size, err := writer.Write(data)
			if err != nil {
				t.Errorf("Failed to write data: %v", err)
			}
			err = writer.Close()
			assert.NoError(t, err, "Failed to write data")
			assert.Equal(t, objectSize, size, "data size not match")
		}
	})

	t.Run("get object", func(t *testing.T) {
		reader := factory.GetEcosReader("/test20/ecos-test")
		data := make([]byte, objectSize)
		size, err := reader.Read(data)
		if err != nil && err != io.EOF {
			t.Errorf("Failed to read data: %v", err)
		}
		assert.Equal(t, objectSize, size, "data size not match")
	})

	t.Run("test get cluster report", func(t *testing.T) {
		operator := client.GetClusterOperator()
		state, err := operator.State()
		assert.NoError(t, err, "Failed to get cluster state")
		t.Log(state)
	})

	t.Run("test list object meta", func(t *testing.T) {
		meta, err := client.ListObjects(ctx, bucketName, "/test1")
		assert.NoError(t, err, "Failed to list objects")
		assert.Equal(t, objectNum, len(meta), "object count not match")
	})

	t.Run("delete object", func(t *testing.T) {
		operator := client.GetVolumeOperator()
		bucket, err := operator.Get("default")
		assert.NoError(t, err, "Failed to get bucket")
		err = bucket.Remove("/test10/ecos-test")
		assert.NoError(t, err, "Failed to remove object")
		_, err = bucket.Get("/test10/ecos-test")
		assert.Error(t, err, "get removed object should fail")
		time.Sleep(time.Second)
	})

	t.Run("list bucket", func(t *testing.T) {
		bucketInfo := infos.GenBucketInfo("root", "test", "root")
		err = client.GetVolumeOperator().CreateBucket(bucketInfo)
		buckets, err := client.GetVolumeOperator().List("")
		assert.NoError(t, err, "Failed to list buckets")
		assert.Equal(t, 2, len(buckets), "bucket count not match")
	})
	t.Run("test metrics", func(t *testing.T) {
		metaPutTime := metrics.GetOrRegisterTimer(watcher.MetricsAlayaMetaPutTimer, nil).Mean()
		t.Logf("meta put time: %v", metaPutTime)
		blockPutTime := metrics.GetOrRegisterTimer(watcher.MetricsGaiaBlockPutTimer, nil).Mean()
		t.Logf("block put time: %v", blockPutTime/float64(time.Second.Nanoseconds()))
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
	watchers, _, _ := RunTestEdgeNodeCluster(b, ctx, true, basePath, 9)

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort

	client, err := client2.New(&conf)
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

	factory, err := client.GetIOFactory(bucketName)
	if err != nil {
		b.Errorf("Failed to get io factory: %v", err)
	}
	sizeMap := orderedmap.NewOrderedMap()
	sizeMap.Set("10K", 10*1024)
	sizeMap.Set("1M", 1024*1024)
	sizeMap.Set("10M", 10*1024*1024)
	sizeMap.Set("100M", 100*1024*1024)

	b.Run("put object", func(b *testing.B) {
		keys := sizeMap.Keys()
		for _, key := range keys {
			size := key.(string)
			value, _ := sizeMap.Get(size)
			objectSize := value.(int)
			b.Run(size, func(b *testing.B) {
				//gen test data
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					b.StopTimer()
					data := genTestData(objectSize)
					b.StartTimer()
					writer := factory.GetEcosWriter("test" + strconv.Itoa(n))
					size, err := writer.Write(data)
					if err != nil {
						b.Errorf("Failed to write data: %v", err)
					}
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
		keys := sizeMap.Keys()
		for _, key := range keys {
			size := key.(string)
			value, _ := sizeMap.Get(size)
			objectSize := value.(int)
			b.Run(size, func(b *testing.B) {
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
					//data, _ = io.ReadAll(reader)
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
