package moon

import (
	"context"
	"github.com/coreos/etcd/raft/raftpb"
	"go.etcd.io/etcd/Godeps/_workspace/src/github.com/golang/glog"
	"go.etcd.io/etcd/raft"
	"time"
)

type node struct {
	id          uint64              //raft节点的id
	ctx         context.Context     //context
	pstore      map[string]string   //用来保存key->value的map
	raftStorage *raft.MemoryStorage //raft需要的内存结构
	cfg         *raft.Config        //raft需要的配置
	raft        raft.Node           //前面提到的node
	ticker      <-chan time.Time    //定时器，提供周期时钟源和超时触发能力
	recv        chan raftpb.Message
}

var (
	bcChans = []chan raftpb.Message{
		make(chan raftpb.Message),
		make(chan raftpb.Message),
		make(chan raftpb.Message),
	}
)

func newNode(id uint64, peers []raft.Peer) *node {
	ctx := context.TODO()
	storage := raft.NewMemoryStorage()
	cfg := raft.Config{
		ID:              id,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	n := &node{
		id:          id,
		ctx:         ctx,
		pstore:      make(map[string]string),
		raftStorage: storage,
		cfg:         &cfg,
		ticker:      time.Tick(time.Microsecond),
		recv:        bcChans[id-1],
	}

	n.raft = raft.StartNode(n.cfg, peers)
	return n
}

func (n *node) send(messages []raftpb.Message) {
	for _, m := range messages {
		glog.Infof(raft.DescribeMessage(m, nil))
		to := m.To
		ch := bcChans[to-1]
		glog.Infof("%d send to %v, type %v", n.id, m.To, m.Type)
		ch <- m
	}
}

func (n *node) run() {
	for {
		select {
		case <-n.ticker:
			n.raft.Tick()
		case rd := <-n.raft.Ready():
			n.raftStorage.Append(rd.Entries)
			go n.send(rd.Messages)
			n.raft.Advance()
		case m := <-n.recv:
			glog.Infof("%d got message from %v to %v, type %v", n.id, m.From, m.To, m.Type)
			n.raft.Step(n.ctx, m)
			glog.Infof("%d status: %v", n.id, n.raft.Status().RaftState)
		}
	}
}
