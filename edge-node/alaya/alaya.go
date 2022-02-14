package alaya

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/pipline"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"github.com/coreos/etcd/raft/raftpb"
)

// Alaya process record & inquire object Mata request
// 阿赖耶处理对象元数据的存储和查询请求
// 一切众生阿赖耶识，本来而有圆满清净，出过于世同于涅槃
type Alaya struct {
	UnimplementedAlayaServer

	PGMessageChans map[uint64]chan raftpb.Message
	PGRaftNode     map[uint64]*Raft
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

func NewAlaya(selfInfo *node.NodeInfo, infoStorage node.InfoStorage, rpcServer *messenger.RpcServer, piplines []*pipline.Pipline) *Alaya {
	a := Alaya{}
	for _, p := range piplines {
		pgID := p.PgId
		a.PGMessageChans[pgID] = make(chan raftpb.Message)
		a.PGRaftNode[pgID] = NewAlayaRaft(selfInfo.RaftId, pgID, p, infoStorage, a.PGMessageChans[pgID])
		a.PGRaftNode[pgID].Run()
	}
	RegisterAlayaServer(rpcServer, &a)
	return &a
}
