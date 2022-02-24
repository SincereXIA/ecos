package moon

import (
	"ecos/cloud/sun"
	"ecos/edge-node/node"
	"ecos/messenger"
	"go.etcd.io/etcd/raft"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestMoon_Register(t *testing.T) {
	sunRpc := messenger.NewRpcServer(3260)
	sun.NewSun(sunRpc)
	go sunRpc.Run()

	groupInfo := []*node.NodeInfo{
		node.NewSelfInfo(0x01, "127.0.0.1", 32671),
		node.NewSelfInfo(0x02, "127.0.0.1", 32672),
		node.NewSelfInfo(0x03, "127.0.0.1", 32673),
	}

	rpcServers := []*messenger.RpcServer{
		messenger.NewRpcServer(32671),
		messenger.NewRpcServer(32672),
		messenger.NewRpcServer(32673),
	}

	var moons []*Moon

	for i := 0; i < 3; i++ {
		moon := NewMoon(groupInfo[i], "127.0.0.1:3260", nil, nil, rpcServers[i])
		moons = append(moons, moon)
		go rpcServers[i].Run()
		go moon.Run()
	}

	time.Sleep(2 * time.Second)

	leader := -1
	for {
		for i := 0; i < 3; i++ {
			if raft.StateLeader == moons[i].raft.Status().RaftState {
				leader = i
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if leader >= 0 {
			t.Logf("leader: %v", leader+1)
			break
		}
	}

	dir, _ := ioutil.ReadDir("./ecos-data/db")
	for _, d := range dir {
		os.RemoveAll(path.Join([]string{"ecos-data/db", d.Name()}...))
	}

}
