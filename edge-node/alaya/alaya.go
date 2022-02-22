package alaya

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"github.com/wxnacy/wgo/arrays"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// Alaya process record & inquire object Mata request
// 阿赖耶处理对象元数据的存储和查询请求
// 一切众生阿赖耶识，本来而有圆满清净，出过于世同于涅槃
type Alaya struct {
	UnimplementedAlayaServer

	ctx    context.Context
	cancel context.CancelFunc

	NodeID         uint64
	PGMessageChans map[uint64]chan raftpb.Message
	PGRaftNode     map[uint64]*Raft

	InfoStorage node.InfoStorage
	MetaStorage MetaStorage
}

func (a *Alaya) RecordObjectMeta(ctx context.Context, meta *ObjectMeta) (*common.Result, error) {
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
	if msgChan, ok := a.PGMessageChans[pgID]; ok {
		msgChan <- *pgMessage.Message
		return &PGRaftMessage{
			PgId:    pgMessage.PgId,
			Message: &raftpb.Message{},
		}, nil
	}
	return nil, errno.PGNotExist
}

func (a *Alaya) stop() {
	a.cancel()
}

// ApplyNewPipelines use new pipelines info to change raft nodes in Alaya
// pipelines must in order (from 1 to n)
func (a *Alaya) ApplyNewPipelines(pipelines []*pipeline.Pipeline) {
	// Delete raft node not in new pipelines
	for pgID, _ := range a.PGRaftNode {
		if -1 == arrays.Contains(pipelines[pgID-1], a.NodeID) {
			delete(a.PGRaftNode, pgID)
			delete(a.PGMessageChans, pgID)
		}
	}
	// Add raft node new in pipelines
	for _, p := range pipelines {
		if -1 == arrays.Contains(p.RaftId, a.NodeID) { // pass when node not in pipline
			continue
		}
		pgID := p.PgId
		if _, ok := a.PGMessageChans[pgID]; ok {
			continue // raft node already exist
		}
		// Add new raft node
		a.PGMessageChans[pgID] = make(chan raftpb.Message)
		a.PGRaftNode[pgID] = NewAlayaRaft(a.NodeID, pgID, p, a.InfoStorage, a.MetaStorage, a.PGMessageChans[pgID])
	}
}

func NewAlaya(selfInfo *node.NodeInfo, infoStorage node.InfoStorage, metaStorage MetaStorage,
	rpcServer *messenger.RpcServer, pipelines []*pipeline.Pipeline) *Alaya {
	ctx, cancel := context.WithCancel(context.Background())
	a := Alaya{
		ctx:            ctx,
		cancel:         cancel,
		PGMessageChans: make(map[uint64]chan raftpb.Message),
		PGRaftNode:     make(map[uint64]*Raft),
		NodeID:         selfInfo.RaftId,
		InfoStorage:    infoStorage,
		MetaStorage:    metaStorage,
	}
	RegisterAlayaServer(rpcServer, &a)
	for _, p := range pipelines {
		if -1 == arrays.Contains(p.RaftId, selfInfo.RaftId) { // pass when node not in pipline
			continue
		}
		pgID := p.PgId
		a.PGMessageChans[pgID] = make(chan raftpb.Message)
		a.PGRaftNode[pgID] = NewAlayaRaft(selfInfo.RaftId, pgID, p, infoStorage, metaStorage, a.PGMessageChans[pgID])
	}
	return &a
}

func (a *Alaya) Run() {
	for _, raftNode := range a.PGRaftNode {
		go raftNode.Run()
	}
}

func (a *Alaya) printPipelineInfo() {
	logger.Infof("AlayaID: %v", a.NodeID)
	for pgID, raftNode := range a.PGRaftNode {
		logger.Infof("PGID: %v, leader: %v", pgID, raftNode.raft.Status().Lead)
	}
}
