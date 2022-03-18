package gaia

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/timestamp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"path"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNewGaia(t *testing.T) {
	var basePaths []string
	storage := node.NewMemoryNodeInfoStorage()
	for i := 0; i < 5; i++ {
		basePaths = append(basePaths, "./ecos-data/gaia-"+strconv.Itoa(i))
	}
	var rpcServers []*messenger.RpcServer
	for i := 0; i < 5; i++ {
		rpcServers = append(rpcServers, messenger.NewRpcServer(uint64(32670+i)))
		info := node.NodeInfo{
			RaftId:   uint64(i + 1),
			Uuid:     uuid.New().String(),
			IpAddr:   "127.0.0.1",
			RpcPort:  uint64(32670 + i),
			Capacity: 10,
		}
		_ = storage.UpdateNodeInfo(&info, timestamp.Now())
	}
	storage.Commit(1)
	storage.Apply()
	for i := 0; i < 5; i++ {
		info, _ := storage.GetNodeInfo(node.ID(i + 1))
		config := Config{basePath: basePaths[i]}
		NewGaia(rpcServers[i], info, storage, &config)
		go func(rpcServer *messenger.RpcServer) {
			err := rpcServer.Run()
			if err != nil {
				t.Errorf("rpcServer run error: %v", err)
				return
			}
		}(rpcServers[i])
	}

	time.Sleep(time.Second)

	pipelines := pipeline.GenPipelines(storage.GetGroupInfo(0), 10, 3)
	wait := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		p := pipelines[i]
		wait.Add(1)
		go func(p *pipeline.Pipeline) {
			uploadBlockTest(t, p, storage, basePaths)
			wait.Done()
		}(p)
	}
	wait.Wait()

	for _, p := range basePaths {
		_ = os.RemoveAll(p)
	}
}

func uploadBlockTest(t *testing.T, p *pipeline.Pipeline, storage node.InfoStorage, basePaths []string) {
	id := p.RaftId[0]
	info, _ := storage.GetNodeInfo(node.ID(id))
	conn, _ := messenger.GetRpcConnByInfo(info)

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
		Term:     1,
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
		Term:     1,
	}}})
	if err != nil {
		t.Errorf("Send stream err: %v", err)
	}

	time.Sleep(1 * time.Second)
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
