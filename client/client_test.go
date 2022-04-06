package main

import (
	"context"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	edgeNodeTest "ecos/edge-node/test"
	"ecos/utils/common"
	"github.com/stretchr/testify/assert"
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

	// Add a test bucket first
	bucketName := "default"
	bucketInfo := infos.GenBucketInfo("root", bucketName, "root")

	_, err := watchers[0].GetMoon().ProposeInfo(ctx, &moon.ProposeInfoRequest{
		Head:     nil,
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	if err != nil {
		t.Errorf("Failed to add bucket: %v", err)
	}

	conf := config.DefaultConfig
	conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
	conf.NodePort = watchers[0].GetSelfInfo().RpcPort

	client, err := New(conf)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
	}

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
