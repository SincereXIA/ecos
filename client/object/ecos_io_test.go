package object

import (
	"context"
	"ecos/client/config"
	edgeNodeTest "ecos/edge-node/test"
	"ecos/utils/common"
	"github.com/stretchr/testify/assert"
	"io"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestEcosWriterAndReader(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	ctx, cancel := context.WithCancel(context.Background())
	basePath := "./ecos-data/"
	t.Logf("Current test filename: %s", filename)
	type args struct {
		objectSize int
		key        string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
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
		{"writer 64M object",
			args{
				1024 * 1024 * 64, // 64M
				"/path/64M_obj",
			},
			false,
		},
	}
	_ = common.InitAndClearPath(basePath)
	watchers, _ := edgeNodeTest.RunTestEdgeNodeCluster(ctx, basePath, 9)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := config.DefaultConfig
			conf.NodeAddr = watchers[0].GetSelfInfo().IpAddr
			conf.NodePort = watchers[0].GetSelfInfo().RpcPort
			factory := NewEcosIOFactory(conf)
			writer := factory.GetEcosWriter(tt.args.key)
			reader := factory.GetEcosReader(tt.args.key)
			data := genTestData(tt.args.objectSize)
			writeSize, err := writer.Write(data)
			assert.NoError(t, err)
			assert.Equal(t, tt.args.objectSize, writeSize)
			assert.NoError(t, writer.Close())
			t.Logf("Upload Finish!")
			if (err != nil) != tt.wantErr {
				t.Errorf("PutObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			readData := make([]byte, tt.args.objectSize)
			readSize, err := reader.Read(readData)
			assert.Equal(t, io.EOF, err)
			assert.Equal(t, tt.args.objectSize, readSize)
			assert.Equal(t, true, reflect.DeepEqual(readData, data))
		})
	}

	t.Cleanup(func() {
		cancel()
		_ = os.RemoveAll(basePath)
	})
}

func genTestData(size int) []byte {
	rand.Seed(time.Now().Unix())
	directSize := 1024 * 1024 * 10
	if size < directSize {
		data := make([]byte, size)
		for idx, _ := range data {
			num := random(65, 90)
			data[idx] = byte(num)
		}
		return data
	}
	d := make([]byte, directSize)
	data := make([]byte, 0, size)
	for size-directSize > 0 {
		data = append(data, d...)
		size = size - directSize
	}
	data = append(data, d[0:size]...)
	return data
}

func random(min, max int) int {
	return rand.Intn(max-min) + min
}
