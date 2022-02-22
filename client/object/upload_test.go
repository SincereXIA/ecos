package object

import (
	"ecos/client/config"
	"ecos/edge-node/gaia"
	"ecos/messenger"
	"runtime"
	"testing"
	"time"
)

func TestPutObject(t *testing.T) {
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
				"./upload_test.go",
				"127.0.0.1:32671",
				"/upload.go",
			},
			false,
		},
	}
	rpcServer := messenger.NewRpcServer(32671)
	gaia.NewGaia(rpcServer)
	go func() {
		err := rpcServer.Run()
		if err != nil {
			t.Errorf("UploadFile() error = %v", err)
		}
	}()
	defer rpcServer.Stop()
	time.Sleep(100 * time.Millisecond) // ensure rpcServer running
	ClientConfig = *config.DefaultConfig
	ClientConfig.UploadTimeoutMs = 1000000
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := PutObject(tt.args.localFilePath, tt.args.server, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("PutObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
