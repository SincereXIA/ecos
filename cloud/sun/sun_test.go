package sun

import (
	"context"
	"ecos/edge-node/node"
	"ecos/messenger"
	"github.com/google/uuid"
	"go.etcd.io/etcd/Godeps/_workspace/src/github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"testing"
	"time"
)

func TestSun_MoonRegister(t *testing.T) {
	rpc := messenger.NewRpcServer(3260)
	NewSun(rpc)
	go rpc.Run()
	time.Sleep(time.Millisecond * 300)

	conn, err := grpc.Dial("127.0.0.1:3260", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Errorf("faild to connect: %v", err)
		return
	}
	defer conn.Close()
	c := NewSunClient(conn)

	node1 := node.NodeInfo{
		RaftId:   0,
		Uuid:     uuid.New().String(),
		IpAddr:   "127.0.0.1",
		RpcPort:  3261,
		Capacity: 0,
	}
	node2 := node.NodeInfo{
		RaftId:   0,
		Uuid:     uuid.New().String(),
		IpAddr:   "127.0.0.1",
		RpcPort:  3261,
		Capacity: 0,
	}
	result1, err := c.MoonRegister(context.Background(), &node1)
	result2, err := c.MoonRegister(context.Background(), &node2)

	assert.False(t, result1.HasLeader)
	assert.True(t, result2.HasLeader)
	assert.Equal(t, node1.Uuid, result2.GroupInfo.LeaderInfo.Uuid)
	assert.NotEqual(t, result1.RaftId, result2.RaftId)

	t.Run("Multi time Register", func(t *testing.T) {
		node3 := node.NodeInfo{
			RaftId:   0,
			Uuid:     node1.Uuid,
			IpAddr:   "127.0.0.1",
			RpcPort:  3261,
			Capacity: 0,
		}
		result1, err := c.MoonRegister(context.Background(), &node1)
		result2, err := c.MoonRegister(context.Background(), &node3)
		assert.Empty(t, err)
		assert.Equal(t, result1.RaftId, result2.RaftId)
	})
}
