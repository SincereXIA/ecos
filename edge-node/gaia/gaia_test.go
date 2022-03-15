package gaia

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"github.com/google/uuid"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestNewGaia(t *testing.T) {
	var basePath []string
	storage := node.NewMemoryNodeInfoStorage()
	for i := 0; i < 5; i++ {
		basePath = append(basePath, "./ecos-data/gaia-"+strconv.Itoa(i))
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
		_ = storage.UpdateNodeInfo(&info)
	}
	storage.Commit(1)
	storage.Apply()
	var gaias []*Gaia
	for i := 0; i < 5; i++ {
		info, _ := storage.GetNodeInfo(node.ID(i + 1))
		gaias = append(gaias, NewGaia(rpcServers[i], info, storage))
		go func(rpcServer *messenger.RpcServer) {
			rpcServer.Run()
		}(rpcServers[i])
	}

	time.Sleep(time.Second)

	pipelines := pipeline.GenPipelines(storage.GetGroupInfo(0), 10, 3)
	p := pipelines[0]
	id := p.RaftId[0]
	info, _ := storage.GetNodeInfo(node.ID(id))
	conn, _ := messenger.GetRpcConnByInfo(info)

	client := NewGaiaClient(conn)
	stream, err := client.UploadBlockData(context.TODO())
	if err != nil {
		t.Errorf("Get Upload stream err: %v", err)
	}

	data := [1024]byte{}
	rand.Read(data[:])

	blockInfo := &object.BlockInfo{
		BlockId:   uuid.New().String(),
		BlockSize: uint64(len(data)),
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

	stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Chunk{Chunk: &Chunk{
		Content: data[:],
	}}})

	stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Chunk{Chunk: &Chunk{
		Content: data[:],
	}}})

	stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Chunk{Chunk: &Chunk{
		Content: data[:],
	}}})

	stream.Send(&UploadBlockRequest{Payload: &UploadBlockRequest_Message{Message: &ControlMessage{
		Code:     ControlMessage_EOF,
		Block:    blockInfo,
		Pipeline: p,
		Term:     1,
	}}})

	time.Sleep(10 * time.Second)
}
