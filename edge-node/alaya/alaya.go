package alaya

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	mapset "github.com/deckarep/golang-set"
	"github.com/wxnacy/wgo/arrays"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"sync"
)

// Alaya process record & inquire object Mata request
// 阿赖耶处理对象元数据的存储和查询请求
// 一切众生阿赖耶识，本来而有圆满清净，出过于世同于涅槃
type Alaya struct {
	UnimplementedAlayaServer

	ctx    context.Context
	cancel context.CancelFunc

	NodeID         uint64
	PGMessageChans sync.Map
	PGRaftNode     map[uint64]*Raft

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

func (a *Alaya) RecordObjectMeta(ctx context.Context, meta *object.ObjectMeta) (*common.Result, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		pgID := meta.PgId
		a.PGRaftNode[pgID].ProposeObjectMeta(meta)
		// TODO: 检查元数据是否同步成功
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
	return nil, errno.PGNotExist
}

func (a *Alaya) Stop() {
	for _, raft := range a.PGRaftNode {
		raft.Stop()
	}
	a.MetaStorage.Close()
	a.cancel()
}

func (a *Alaya) ApplyNewGroupInfo(groupInfo *node.GroupInfo) {
	// TODO: (zhang) make pgNum configurable
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
	for pgID, raftNode := range a.PGRaftNode {
		if raftNode.raft.Status().Lead != a.NodeID {
			continue
		}
		// if this node is leader, add new pipelines node first
		p := pipelines[pgID-1]
		needAdd, needRemove := calDiff(p.RaftId, oldPipelines[pgID-1].RaftId)
		err := raftNode.ProposeNewNodes(needAdd)
		if err != nil {
			logger.Errorf("Alaya propose new nodes in PG: %v fail, err: %v", pgID, err)
		}
		go func(node *Raft) {
			err := node.ProposeRemoveNodes(needRemove)
			if err != nil {
				logger.Errorf("Alaya propose remove nodes in PG: %v fail, err: %v", pgID, err)
			}
		}(raftNode)
	}

	// start run raft node new in pipelines
	for _, p := range pipelines {
		if -1 == arrays.Contains(p.RaftId, a.NodeID) { // pass when node not in pipeline
			continue
		}
		pgID := p.PgId
		if _, ok := a.PGMessageChans.Load(pgID); ok {
			continue // raft node already exist
		}
		// Add new raft node
		var raft *Raft
		if oldPipelines != nil {
			raft = a.MakeAlayaRaftInPipeline(pgID, p, oldPipelines[pgID-1].RaftId[0])
		} else {
			raft = a.MakeAlayaRaftInPipeline(pgID, p, 0)
		}
		if a.state == UPDATING {
			go raft.Run()
		}
	}
}

func NewAlaya(selfInfo *node.NodeInfo, infoStorage node.InfoStorage, metaStorage MetaStorage,
	rpcServer *messenger.RpcServer) *Alaya {
	ctx, cancel := context.WithCancel(context.Background())
	a := Alaya{
		ctx:            ctx,
		cancel:         cancel,
		PGMessageChans: sync.Map{},
		PGRaftNode:     make(map[uint64]*Raft),
		NodeID:         selfInfo.RaftId,
		InfoStorage:    infoStorage,
		MetaStorage:    metaStorage,
		state:          INIT,
	}
	RegisterAlayaServer(rpcServer, &a)
	a.ApplyNewGroupInfo(infoStorage.GetGroupInfo(0))
	infoStorage.SetOnGroupApply(a.ApplyNewGroupInfo)
	a.state = READY
	return &a
}

func (a *Alaya) Run() {
	for _, raftNode := range a.PGRaftNode {
		go raftNode.Run()
	}
	a.state = RUNNING
}

func (a *Alaya) printPipelineInfo() {
	for pgID, raftNode := range a.PGRaftNode {
		logger.Infof("Alaya: %v, PG: %v, leader: %v", a.NodeID, pgID, raftNode.raft.Status().Lead)
	}
}

// MakeAlayaRaftInPipeline Make a new raft node for a single pipeline (PG), it will:
//
// 1. Create a raft message chan if not exist
//
// 2. New an alaya raft node **but not run it**
//
// when leaderID is not zero, it while add to an existed raft group
func (a *Alaya) MakeAlayaRaftInPipeline(pgID uint64, p *pipeline.Pipeline, leaderID uint64) *Raft {
	c, ok := a.PGMessageChans.Load(pgID)
	if !ok {
		c = make(chan raftpb.Message)
		a.PGMessageChans.Store(pgID, c)
	}
	a.PGRaftNode[pgID] = NewAlayaRaft(a.NodeID, pgID, p, leaderID, a.InfoStorage, a.MetaStorage, c.(chan raftpb.Message))
	if p.RaftId[0] == a.NodeID {
		go a.PGRaftNode[pgID].RunAskForLeader()
	}
	logger.Infof("Node: %v successful add raft node in alaya, PG: %v", a.NodeID, pgID)
	return a.PGRaftNode[pgID]
}

func calDiff(a []uint64, b []uint64) (da []uint64, db []uint64) {
	setA := mapset.NewSet()
	for _, n := range a {
		setA.Add(n)
	}
	setB := mapset.NewSet()
	for _, n := range b {
		setB.Add(n)
	}
	for num := range setA.Difference(setB).Iter() {
		da = append(da, num.(uint64))
	}
	for num := range setB.Difference(setA).Iter() {
		db = append(db, num.(uint64))
	}
	return
}
