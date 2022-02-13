package alaya

import (
	"context"
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
}

func (a *Alaya) RecordObjectMeta(ctx context.Context, meta *ObjectMeta) (*common.Result, error) {
	// TODO: 处理收到的元数据，转发给同组 Node
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return nil, nil
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

func NewAlaya(rpcServer *messenger.RpcServer) *Alaya {
	a := Alaya{}
	RegisterAlayaServer(rpcServer, &a)
	return &a
}
