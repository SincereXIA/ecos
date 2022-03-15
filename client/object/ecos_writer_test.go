package object

import (
	"ecos/client/config"
	"ecos/edge-node/gaia"
	"ecos/messenger"
	"github.com/stretchr/testify/assert"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestEcosWriter(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	t.Logf("Current test filename: %s", filename)
	type args struct {
		localFilePath string
		server        string
		key           string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"Gaia Test",
			args{
				"./ecos_writer_test.go",
				"127.0.0.1:32801",
				"/upload.go",
			},
			false,
		},
	}
	rpcServer1 := messenger.NewRpcServer(32801)
	gaia.NewGaia(rpcServer1)
	go func() {
		err := rpcServer1.Run()
		if err != nil {
			t.Errorf("UploadFile() error = %v", err)
		}
	}()

	rpcServer2 := messenger.NewRpcServer(32802)
	gaia.NewGaia(rpcServer2)
	go func() {
		err := rpcServer2.Run()
		if err != nil {
			t.Errorf("UploadFile() error = %v", err)
		}
	}()

	rpcServer3 := messenger.NewRpcServer(32803)
	gaia.NewGaia(rpcServer3)
	go func() {
		err := rpcServer3.Run()
		if err != nil {
			t.Errorf("UploadFile() error = %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond) // ensure rpcServer running

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewEcosWriterFactory(config.DefaultConfig, "127.0.0.1", 32801)
			writer := factory.GetEcosWriter(tt.args.key)
			file, err := os.Open(tt.args.localFilePath)
			assert.NoError(t, err)
			for {
				var data = make([]byte, 1<<22)
				readSize, err := file.Read(data)
				if err == io.EOF {
					break
				}
				assert.NoError(t, err)
				writeSize, err := writer.Write(data[:readSize])
				assert.NoError(t, err)
				assert.Equal(t, readSize, writeSize)
			}
			assert.NoError(t, writer.Close())
			t.Logf("Upload Finish!")
			if (err != nil) != tt.wantErr {
				t.Errorf("PutObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			time.Sleep(1 * time.Second)
		})
	}
	rpcServer1.Stop()
	rpcServer2.Stop()
	rpcServer3.Stop()
}

func TestPortClose(t *testing.T) {
	for port := 32801; port < 32804; port++ {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second)
		if err == nil && conn != nil {
			t.Errorf("port %v not close!", port)
		}
	}
	t.Log("All ports closed!")
}
