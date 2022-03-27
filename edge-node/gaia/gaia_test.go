package gaia

import (
	"context"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"path"
	"strconv"
	"sync"
	"testing"
)

func TestNewGaia(t *testing.T) {
	basePath := "./ecos-data/"
	var basePaths []string
	nodeNum := 5
	ctx := context.Background()

	watchers, rpcServers, _ := watcher.GenTestWatcherCluster(ctx, basePath, nodeNum)

	for i := 0; i < 5; i++ {
		basePaths = append(basePaths, "./ecos-data/gaia-"+strconv.Itoa(i))
	}

	for i := 0; i < 5; i++ {
		info := watchers[i].GetSelfInfo()
		config := Config{BasePath: basePaths[i]}
		NewGaia(ctx, rpcServers[i], info, watchers[i], &config)
		go func(rpcServer *messenger.RpcServer) {
			err := rpcServer.Run()
			if err != nil {
				t.Errorf("rpcServer run error: %v", err)
				return
			}
		}(rpcServers[i])
	}

	watcher.RunAllTestWatcher(watchers)

	t.Cleanup(func() {
		for _, server := range rpcServers {
			server.Stop()
		}

		for _, p := range basePaths {
			_ = os.RemoveAll(p)
		}
	})

	watcher.WaitAllTestWatcherOK(watchers)

	pipelines := pipeline.GenPipelines(watchers[0].GetCurrentClusterInfo(), 10, 3)
	wait := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		p := pipelines[i]
		wait.Add(1)
		go func(p *pipeline.Pipeline) {
			uploadBlockTest(t, p, watchers, basePaths)
			wait.Done()
		}(p)
	}
	wait.Wait()
}

func uploadBlockTest(t *testing.T, p *pipeline.Pipeline, watchers []*watcher.Watcher, basePaths []string) {
	id := p.RaftId[0]
	info := watchers[id-1].GetSelfInfo()
	term := watchers[id-1].GetCurrentClusterInfo().Term
	conn, _ := messenger.GetRpcConnByNodeInfo(info)

	client := NewGaiaClient(conn)
	stream, err := client.UploadBlockData(context.TODO())
	if err != nil {
		t.Errorf("Get Upload stream err: %v", err)
	}

	testTrunkSize := 1024 * 1024
	testBlockSize := testTrunkSize * 8

	data := make([]byte, testTrunkSize)
	rand.Read(data[:])

	blockInfo := &object.BlockInfo{
		BlockId:   uuid.New().String(),
		BlockSize: uint64(testBlockSize),
		BlockHash: "",
		PgId:      1,
	}

	err = stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Message{Message: &ControlMessage{
		Code:     ControlMessage_BEGIN,
		Block:    blockInfo,
		Pipeline: p,
		Term:     term,
	}}})
	if err != nil {
		t.Errorf("Send stream err: %v", err)
	}

	for i := 0; i < testBlockSize/testTrunkSize; i++ {
		err = stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Chunk{Chunk: &Chunk{
			Content: data[:],
		}}})
		if err != nil {
			t.Errorf("Send stream err: %v", err)
		}
	}

	err = stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Message{Message: &ControlMessage{
		Code:     ControlMessage_EOF,
		Block:    blockInfo,
		Pipeline: p,
		Term:     term,
	}}})
	if err != nil {
		t.Errorf("Send stream err: %v", err)
	}
	result, err := stream.CloseAndRecv()
	if err != nil || result.Status != common.Result_OK {
		t.Errorf("receive close stream message fail: %v", err)
	}
	assertFilesOK(t, blockInfo.BlockId, blockInfo.BlockSize, p, basePaths)
}

func assertFilesOK(t *testing.T, blockID string, fileSize uint64, p *pipeline.Pipeline, basePaths []string) {
	var paths []string
	for _, id := range p.RaftId {
		paths = append(paths, path.Join(basePaths[id-1], blockID))
	}

	for _, filePath := range paths {
		stat, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				t.Errorf("path not exist: %v", filePath)
			}
			t.Errorf("path not ok: %v", filePath)
			return
		}
		assert.Equal(t, fileSize, uint64(stat.Size()), "file size should be equal")
	}
}
