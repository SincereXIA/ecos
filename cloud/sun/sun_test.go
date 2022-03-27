package sun

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSun_MoonRegister(t *testing.T) {
	port, rpc := messenger.NewRandomPortRpcServer()
	NewSun(rpc)
	go rpc.Run()
	time.Sleep(time.Millisecond * 300)

	conn, err := messenger.GetRpcConn("127.0.0.1", port)
	if err != nil {
		t.Errorf("faild to connect: %v", err)
		return
	}
	defer conn.Close()
	c := NewSunClient(conn)

	node1 := infos.NodeInfo{
		RaftId:   0,
		Uuid:     uuid.New().String(),
		IpAddr:   "127.0.0.1",
		RpcPort:  port,
		Capacity: 0,
	}
	node2 := infos.NodeInfo{
		RaftId:   0,
		Uuid:     uuid.New().String(),
		IpAddr:   "127.0.0.1",
		RpcPort:  port,
		Capacity: 0,
	}
	result1, err := c.MoonRegister(context.Background(), &node1)
	result2, err := c.MoonRegister(context.Background(), &node2)

	assert.False(t, result1.HasLeader)
	assert.True(t, result2.HasLeader)
	assert.Equal(t, node1.Uuid, result2.ClusterInfo.LeaderInfo.Uuid)
	assert.NotEqual(t, result1.RaftId, result2.RaftId)

	t.Run("Multi time Register", func(t *testing.T) {
		node3 := infos.NodeInfo{
			RaftId:   0,
			Uuid:     node1.Uuid,
			IpAddr:   "127.0.0.1",
			RpcPort:  port,
			Capacity: 0,
		}
		result1, err := c.MoonRegister(context.Background(), &node1)
		result2, err := c.MoonRegister(context.Background(), &node3)
		assert.Empty(t, err)
		assert.Equal(t, result1.RaftId, result2.RaftId)
	})
}
