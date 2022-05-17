package io

import (
	"bytes"
	"context"
	"ecos/client/config"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	edgeNodeTest "ecos/edge-node/test"
	"ecos/utils/common"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestEcosWriterAndReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"
	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})

	type args struct {
		objectSize int
		key        string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"write 0.1M object",
			args{
				objectSize: 1024 * 102, // 0.1M
				key:        "test-object-0.1M",
			},
			false,
		},
		{"writer 8M object",
			args{
				1024 * 1024 * 8, // 8M
				"/path/8M_obj",
			},
			false,
		},
		{"writer 8.1M object",
			args{
				1024*1024*8 + 1024*100, // 8.1M
				"/path/8.1M_obj",
			},
			false,
		},
		{"writer 32M object",
			args{
				1024 * 1024 * 32, // 32M
				"/path/32M_obj",
			},
			false,
		},
	}
	_ = common.InitAndClearPath(basePath)
	watchers, _ := edgeNodeTest.RunTestEdgeNodeCluster(t, ctx, false, basePath, 9)

	// Add a test bucket first
	bucketInfo := infos.GenBucketInfo("root", "default", "root")
	_, err := watchers[0].GetMoon().ProposeInfo(ctx, &moon.ProposeInfoRequest{
		Head:     nil,
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	if err != nil {
		t.Errorf("Failed to add bucket: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.DefaultConfig
			conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
			conf.NodePort = watchers[0].GetSelfInfo().RpcPort
			factory, _ := NewEcosIOFactory(&conf, "root", "default")
			data := genTestData(tt.args.objectSize)
			testBigBufferWriteRead(t, tt.args.key+"big", data, factory)
			testSmallBufferWriteRead(t, tt.args.key+"small", data, factory, 1024*1024)
		})
	}

}

func testBigBufferWriteRead(t *testing.T, key string, data []byte, factory *EcosIOFactory) {
	writer := factory.GetEcosWriter(key)
	writeSize, err := writer.Write(data)
	assert.NoError(t, err)
	assert.Equal(t, len(data), writeSize)
	assert.NoError(t, writer.Close())
	t.Logf("Upload key: %v Finish", key)
	if err != nil {
		t.Errorf("PutObject() error = %v", err)
		return
	}

	reader := factory.GetEcosReader(key)
	readData := make([]byte, len(data))
	readSize, err := reader.Read(readData)
	t.Logf("get key: %v Finish", key)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, len(data), readSize, "result size not equal to data size")
	assert.True(t, bytes.Equal(data, readData))
}

func testSmallBufferWriteRead(t *testing.T, key string, data []byte, factory *EcosIOFactory, bufferSize int) {
	writer := factory.GetEcosWriter(key)
	writeBuffer := make([]byte, bufferSize)
	pending := len(data)
	count := 0
	for pending > 0 {
		count++
		wantSize := copy(writeBuffer, data[len(data)-pending:])
		writeSize, err := writer.Write(writeBuffer[:wantSize])
		assert.NoError(t, err)
		assert.Equal(t, wantSize, writeSize)
		assert.Equal(t, count, writer.chunkCount)
		pending -= writeSize
	}
	assert.NoError(t, writer.Close())
	t.Logf("Upload key: %v Finish", key)

	reader := factory.GetEcosReader(key)
	readBuffer := make([]byte, bufferSize)
	result := make([]byte, 0, len(data))
	for {
		readSize, err := reader.Read(readBuffer)
		result = append(result, readBuffer[:readSize]...)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Errorf("Read() error = %v", err)
		}
	}
	t.Logf("get key: %v Finish", key)
	assert.Equal(t, len(data), len(result), "result size not equal to data size")
	assert.True(t, bytes.Equal(data, result))
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
