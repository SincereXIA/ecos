package moon

import (
	"github.com/coreos/etcd/raft/raftpb"
	"go.etcd.io/etcd/raft"
	"reflect"
	"testing"
	"time"
)

func TestRaft(t *testing.T) {

	groupInfo := []*NodeInfo{
		NewSelfInfo(0x01, "127.0.0.1", 32671),
		NewSelfInfo(0x02, "127.0.0.1", 32672),
		NewSelfInfo(0x03, "127.0.0.1", 32673),
	}

	node1 := NewNode(groupInfo[0], nil, groupInfo)
	go node1.run()
	node2 := NewNode(groupInfo[1], nil, groupInfo)
	go node2.run()
	node3 := NewNode(groupInfo[2], nil, groupInfo)
	go node3.run()

	nodes := []*node{node1, node2, node3}

	for {
		leader := -1
		for i := 0; i < 3; i++ {
			if raft.StateLeader == nodes[i].raft.Status().RaftState {
				leader = i
				break
			}
			time.Sleep(100 * time.Microsecond)
		}
		if leader >= 0 {
			t.Logf("leader: %v", leader+1)
			break
		}
	}

	for i := 0; i < 3; i++ {
		nodes[i].reportSelfInfo()
	}

	node4Info := NewSelfInfo(0x04, "127.0.0.1", 32674)
	_ = node1.raft.ProposeConfChange(node1.ctx, raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  0x04,
		Context: nil,
	})

	time.Sleep(2 * time.Second)

	node4 := NewNode(node4Info, nil, groupInfo)
	go node4.run()

	nodes = append(nodes, node4)
	node4.reportSelfInfo()

	time.Sleep(2 * time.Second)
	info := nodes[0].infoStorage.ListAllNodeInfo()
	t.Log(info)
	for i := 1; i < 4; i++ {
		anotherInfo := nodes[i].infoStorage.ListAllNodeInfo()
		if !reflect.DeepEqual(info, anotherInfo) {
			t.Errorf("Node Info Not Equal")
		}
		t.Log(anotherInfo)
	}
}

func TestRaftSingle(t *testing.T) {

	groupInfo := []*NodeInfo{
		NewSelfInfo(0x01, "127.0.0.1", 32671),
	}
	node1 := NewNode(groupInfo[0], nil, groupInfo)
	go node1.run()
	time.Sleep(5 * time.Second)
}
