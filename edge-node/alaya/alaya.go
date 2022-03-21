package alaya

import (
	"context"
	"ecos/edge-node/moon"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"github.com/wxnacy/wgo/arrays"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"sync"
	"time"
)

// Alaya process record & inquire object Mata request
// 阿赖耶处理对象元数据的存储和查询请求
// 一切众生阿赖耶识，本来而有圆满清净，出过于世同于涅槃
type Alaya struct {
	UnimplementedAlayaServer

	ctx    context.Context
	cancel context.CancelFunc

	SelfInfo       *node.NodeInfo
	PGMessageChans sync.Map
	// PGRaftNode save all raft node on this edge.
	// pgID -> Raft node
	PGRaftNode sync.Map
	// mutex ensure Alaya.Run after pipeline ready
	mutex            sync.Mutex
	raftNodeStopChan chan uint64 // 内部 raft node 主动退出时，使用该 chan 通知 alaya

	InfoStorage node.InfoStorage
	MetaStorage MetaStorage

	state State

	pipelines []*pipeline.Pipeline
}

type State int

const (
	INIT State = iota
	READY
	RUNNING
	UPDATING
	STOPPED
)

func (a *Alaya) getRaftNode(pgID uint64) *Raft {
	raft, ok := a.PGRaftNode.Load(pgID)
	if !ok {
		return nil
	}
	return raft.(*Raft)
}

func (a *Alaya) RecordObjectMeta(ctx context.Context, meta *object.ObjectMeta) (*common.Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		pgID := meta.PgId
		a.getRaftNode(pgID).ProposeObjectMeta(meta)
		// TODO: 检查元数据是否同步成功
		logger.Infof("Alaya record object meta success, obj_id: %v, size: %v", meta.ObjId, meta.ObjSize)
	}

	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (a *Alaya) SendRaftMessage(ctx context.Context, pgMessage *PGRaftMessage) (*PGRaftMessage, error) {
	pgID := pgMessage.PgId
	if msgChan, ok := a.PGMessageChans.Load(pgID); ok {
		msgChan.(chan raftpb.Message) <- *pgMessage.Message
		return &PGRaftMessage{
			PgId:    pgMessage.PgId,
			Message: &raftpb.Message{},
		}, nil
	}
	logger.Warningf("receive raft message from: %v, but pg: %v not exist", pgMessage.Message.From, pgID)
	return nil, errno.PGNotExist
}

func (a *Alaya) Stop() {
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		go value.(*Raft).Stop()
		return true
	})
	a.MetaStorage.Close()
	a.cancel()
}

func (a *Alaya) ApplyNewGroupInfo(groupInfo *node.GroupInfo) {
	// TODO: (zhang) make pgNum configurable
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.state = UPDATING
	oldPipelines := a.pipelines
	terms := a.InfoStorage.GetTermList()
	if len(terms) > 0 {
		oldGroupInfo := a.InfoStorage.GetGroupInfo(terms[len(terms)-1])
		oldPipelines = pipeline.GenPipelines(oldGroupInfo, 10, 3)
	}
	if len(groupInfo.NodesInfo) == 0 {
		logger.Warningf("Empty groupInfo when alaya apply new groupInfo")
		return
	}
	p := pipeline.GenPipelines(groupInfo, 10, 3)
	a.ApplyNewPipelines(p, oldPipelines)
	a.state = RUNNING
}

// ApplyNewPipelines use new pipelines info to change raft nodes in Alaya
// pipelines must in order (from 1 to n)
func (a *Alaya) ApplyNewPipelines(pipelines []*pipeline.Pipeline, oldPipelines []*pipeline.Pipeline) {
	a.pipelines = pipelines

	// Add raft node new in pipelines
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		raftNode := value.(*Raft)
		pgID := key.(uint64)

		if raftNode.raft.Status().Lead != a.SelfInfo.RaftId {
			return true
		}
		// if this node is leader, add new pipelines node first
		p := pipelines[pgID-1]
		raftNode.ProposeNewPipeline(p, oldPipelines[pgID-1])
		return true
	})

	// start run raft node new in pipelines
	for _, p := range pipelines {
		if -1 == arrays.Contains(p.RaftId, a.SelfInfo.RaftId) { // pass when node not in pipeline
			continue
		}
		pgID := p.PgId
		if _, ok := a.PGMessageChans.Load(pgID); ok {
			continue // raft node already exist
		}
		// Add new raft node
		var raft *Raft
		if oldPipelines != nil {
			raft = a.MakeAlayaRaftInPipeline(p, oldPipelines[pgID-1])
		} else {
			raft = a.MakeAlayaRaftInPipeline(p, nil)
		}
		if a.state == UPDATING {
			go func(raft *Raft) {
				time.Sleep(time.Second * 2)
				raft.Run()
			}(raft)
		}
	}
}

// NewAlayaByMoon will wait moon ready then create a new alaya
func NewAlayaByMoon(moon *moon.Moon, storage MetaStorage, server *messenger.RpcServer) *Alaya {
	return NewAlaya(moon.SelfInfo, moon.InfoStorage, storage, server)
}

func NewAlaya(selfInfo *node.NodeInfo, infoStorage node.InfoStorage, metaStorage MetaStorage,
	rpcServer *messenger.RpcServer) *Alaya {
	ctx, cancel := context.WithCancel(context.Background())
	a := Alaya{
		ctx:              ctx,
		cancel:           cancel,
		PGMessageChans:   sync.Map{},
		PGRaftNode:       sync.Map{},
		SelfInfo:         selfInfo,
		InfoStorage:      infoStorage,
		MetaStorage:      metaStorage,
		state:            INIT,
		raftNodeStopChan: make(chan uint64),
	}
	RegisterAlayaServer(rpcServer, &a)
	a.ApplyNewGroupInfo(infoStorage.GetGroupInfo(0))
	infoStorage.SetOnGroupApply(a.ApplyNewGroupInfo)
	a.state = READY
	return &a
}

func (a *Alaya) Run() {
	a.mutex.Lock()
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		go value.(*Raft).Run()
		return true
	})
	a.state = RUNNING
	a.mutex.Unlock()

	for {
		select {
		case pgID := <-a.raftNodeStopChan:
			a.PGMessageChans.Delete(pgID)
			a.PGRaftNode.Delete(pgID)
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *Alaya) printPipelineInfo() {
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		raftNode := value.(*Raft)
		pgID := key.(uint64)
		logger.Infof("Alaya: %v, PG: %v, leader: %v, voter: %v", a.SelfInfo.RaftId, pgID, raftNode.raft.Status().Lead, raftNode.GetVotersID())
		return true
	})
}

// MakeAlayaRaftInPipeline Make a new raft node for a single pipeline (PG), it will:
//
// 1. Create a raft message chan if not exist
//
// 2. New an alaya raft node **but not run it**
//
// when leaderID is not zero, it while add to an existed raft group
func (a *Alaya) MakeAlayaRaftInPipeline(p *pipeline.Pipeline, oldP *pipeline.Pipeline) *Raft {
	pgID := p.PgId
	c, ok := a.PGMessageChans.Load(pgID)
	if !ok {
		c = make(chan raftpb.Message)
		a.PGMessageChans.Store(pgID, c)
	}
	a.PGRaftNode.Store(pgID,
		NewAlayaRaft(a.SelfInfo.RaftId, p, oldP, a.InfoStorage, a.MetaStorage,
			c.(chan raftpb.Message), a.raftNodeStopChan))
	logger.Infof("Node: %v successful add raft node in alaya, PG: %v", a.SelfInfo.RaftId, pgID)
	return a.getRaftNode(pgID)
}
