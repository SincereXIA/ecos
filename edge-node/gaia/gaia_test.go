package gaia

import (
	"context"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/gaia"
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
		basePaths = append(basePaths, "./ecos-data/gaia-"+strconv.Itoa(i+1))
	}

	for i := 0; i < 5; i++ {
		config := Config{BasePath: basePaths[i]}
		NewGaia(ctx, rpcServers[i], watchers[i], &config)
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

	// Test upload blocks
	blockInfoMap := sync.Map{}
	wait := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		p := pipelines[i]
		wait.Add(1)
		go func(p *pipeline.Pipeline, pgID uint64) {
			blockInfo := uploadBlockTest(t, p, pgID+1, watchers, basePaths)
			blockInfoMap.Store(blockInfo.BlockId, blockInfo)
			wait.Done()
		}(p, uint64(i))
	}
	wait.Wait()

	// Test delete blocks
	blockInfoMap.Range(func(key, value interface{}) bool {
		blockInfo := value.(*object.BlockInfo)
		p := pipelines[blockInfo.PgId-1] // pipeline id start from 1
		err := deleteBlock(p, watchers, blockInfo)
		if err != nil {
			t.Errorf("delete block error: %v", err)
		}
		assertFilesEmpty(t, blockInfo.BlockId, basePaths)
		return true
	})

	// Test delete not exist block
	t.Run("delete not exist block", func(t *testing.T) {
		p := pipelines[0]
		blockInfo := &object.BlockInfo{
			BlockId: uuid.New().String(),
			PgId:    1,
		}
		err := deleteBlock(p, watchers, blockInfo)
		if err == nil {
			t.Errorf("delete block should be error")
		}
	})
}

func uploadBlockTest(t *testing.T, p *pipeline.Pipeline, pgID uint64,
	watchers []*watcher.Watcher, basePaths []string) *object.BlockInfo {
	id := p.RaftId[0]
	info := watchers[id-1].GetSelfInfo()
	term := watchers[id-1].GetCurrentClusterInfo().Term
	conn, _ := messenger.GetRpcConnByNodeInfo(info)

	client := gaia.NewGaiaClient(conn)
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
		PgId:      pgID,
	}

	err = stream.Send(&gaia.UploadBlockRequest{Payload: &gaia.UploadBlockRequest_Message{Message: &gaia.ControlMessage{
		Code:     gaia.ControlMessage_BEGIN,
		Block:    blockInfo,
		Pipeline: p,
		Term:     term,
	}}})
	if err != nil {
		t.Errorf("Send stream err: %v", err)
	}

	for i := 0; i < testBlockSize/testTrunkSize; i++ {
		err = stream.Send(&gaia.UploadBlockRequest{Payload: &gaia.UploadBlockRequest_Chunk{Chunk: &gaia.Chunk{
			Content: data[:],
		}}})
		if err != nil {
			t.Errorf("Send stream err: %v", err)
		}
	}

	err = stream.Send(&gaia.UploadBlockRequest{Payload: &gaia.UploadBlockRequest_Message{Message: &gaia.ControlMessage{
		Code:     gaia.ControlMessage_EOF,
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
	return blockInfo
}

func deleteBlock(p *pipeline.Pipeline, watchers []*watcher.Watcher, blockInfo *object.BlockInfo) error {
	id := p.RaftId[0]
	info := watchers[id-1].GetSelfInfo()
	term := watchers[id-1].GetCurrentClusterInfo().Term
	conn, _ := messenger.GetRpcConnByNodeInfo(info)

	client := gaia.NewGaiaClient(conn)

	_, err := client.DeleteBlock(context.TODO(), &gaia.DeleteBlockRequest{
		BlockId:  blockInfo.BlockId,
		Term:     term,
		Pipeline: p,
	})
	return err
}

func assertFilesEmpty(t *testing.T, blockID string, basePaths []string) {
	for _, basePath := range basePaths {
		blockPath := path.Join(basePath, blockID)
		if _, err := os.Stat(blockPath); err == nil {
			t.Errorf("block file %s should not exist", blockPath)
		}
	}
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
			continue
		}
		t.Logf("ok: %v", filePath)
		assert.Equal(t, fileSize, uint64(stat.Size()), "file size should be equal")
	}
}
