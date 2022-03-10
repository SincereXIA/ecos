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
	p := pipeline.GenPipelines(groupInfo, 10, 3)
	a.ApplyNewPipelines(p)
	a.state = RUNNING
}

// ApplyNewPipelines use new pipelines info to change raft nodes in Alaya
// pipelines must in order (from 1 to n)
func (a *Alaya) ApplyNewPipelines(pipelines []*pipeline.Pipeline) {
	//
	// Delete raft node not in new pipelines
	for pgID, raftNode := range a.PGRaftNode {
		if -1 == arrays.Contains(pipelines[pgID-1], a.NodeID) {
			go func() {
				err := raftNode.Leave()
				if err != nil {
					logger.Errorf("Alaya raftNode leave fail, PG: %v, err: %v", pgID)
				}
				delete(a.PGRaftNode, pgID)
				a.PGMessageChans.Delete(pgID)
			}()
		}
	}
	// Add raft node new in pipelines
	for _, p := range pipelines {
		if -1 == arrays.Contains(p.RaftId, a.NodeID) { // pass when node not in pipeline
			continue
		}
		pgID := p.PgId
		if _, ok := a.PGMessageChans.Load(pgID); ok {
			continue // raft node already exist
		}
		// Add new raft node
		raft := a.MakeAlayaRaftInPipeline(pgID, p)
		if a.state == UPDATING {
			go raft.Run()
		}
	}
}

func NewAlaya(selfInfo *node.NodeInfo, infoStorage node.InfoStorage, metaStorage MetaStorage,
	rpcServer *messenger.RpcServer, pipelines []*pipeline.Pipeline) *Alaya {
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
	a.ApplyNewPipelines(pipelines)
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

func (a *Alaya) MakeAlayaRaftInPipeline(pgID uint64, p *pipeline.Pipeline) *Raft {
	c, ok := a.PGMessageChans.Load(pgID)
	if !ok {
		c = make(chan raftpb.Message)
		a.PGMessageChans.Store(pgID, c)
	}
	a.PGRaftNode[pgID] = NewAlayaRaft(a.NodeID, pgID, p, a.InfoStorage, a.MetaStorage, c.(chan raftpb.Message))
	if p.RaftId[0] == a.NodeID {
		go a.PGRaftNode[pgID].RunAskForLeader()
	}
	logger.Infof("Node: %v successful add raft node in alaya, PG: %v", a.NodeID, pgID)
	return a.PGRaftNode[pgID]
}
