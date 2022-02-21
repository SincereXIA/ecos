package alaya

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/pipeline"
	"ecos/utils/logger"
	"github.com/coreos/etcd/raft/raftpb"
	"go.etcd.io/etcd/raft"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"strconv"
	"time"
)

type Raft struct {
	pgID        uint64
	ctx         context.Context //context
	InfoStorage node.InfoStorage
	raftStorage *raft.MemoryStorage //raft需要的内存结构
	raftCfg     *raft.Config        //raft需要的配置
	raft        raft.Node
	ticker      <-chan time.Time //定时器，提供周期时钟源和超时触发能力

	raftChan chan raftpb.Message

	metaStorage MetaStorage
}

func NewAlayaRaft(raftID uint64, pgID uint64, pipline *pipeline.Pipeline,
	infoStorage node.InfoStorage,
	raftChan chan raftpb.Message) *Raft {

	ctx := context.Background()
	raftStorage := raft.NewMemoryStorage()

	r := &Raft{
		pgID:        pgID,
		ctx:         ctx,
		InfoStorage: infoStorage,
		raftStorage: raftStorage,
		raftCfg: &raft.Config{
			ID:              raftID,
			ElectionTick:    10,
			HeartbeatTick:   1,
			Storage:         raftStorage,
			MaxSizePerMsg:   4096,
			MaxInflightMsgs: 256,
		},
		ticker:   time.Tick(time.Millisecond * 100),
		raftChan: raftChan,
	}

	var peers []raft.Peer
	for _, id := range pipline.RaftId {
		peers = append(peers, raft.Peer{
			ID:      id,
			Context: nil,
		})
	}
	r.raft = raft.StartNode(r.raftCfg, peers)
	return r
}

func (r *Raft) Run() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-r.ticker:
			r.raft.Tick()
		case ready := <-r.raft.Ready():
			_ = r.raftStorage.Append(ready.Entries)
			go r.sendMsgByRpc(ready.Messages)
			for _, entry := range ready.CommittedEntries {
				r.process(entry)
				if entry.Type == raftpb.EntryConfChange {
					var cc raftpb.ConfChange
					_ = cc.Unmarshal(entry.Data)
					r.raft.ApplyConfChange(cc)
				}
			}
			r.raft.Advance()
		case message := <-r.raftChan:
			_ = r.raft.Step(r.ctx, message)
		}
	}
}

func (r *Raft) sendMsgByRpc(messages []raftpb.Message) {
	for _, message := range messages {
		nodeId := node.ID(message.To)
		nodeInfo, err := r.InfoStorage.GetNodeInfo(nodeId)
		port := strconv.FormatUint(nodeInfo.RpcPort, 10)
		// TODO: save grpc connection
		conn, err := grpc.Dial(nodeInfo.IpAddr+":"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warningf("faild to connect: %v", err)
			continue
		}
		defer func(conn *grpc.ClientConn) {
			err = conn.Close()
			if err != nil {
				logger.Warningf("close grpc conn err: %v", err)
			}
		}(conn)
		c := NewAlayaClient(conn)
		_, err = c.SendRaftMessage(r.ctx, &PGRaftMessage{
			PgId:    r.pgID,
			Message: &message,
		})
		if err != nil {
			logger.Warningf("alaya send raft message by rpc err: %v", err)
		}
	}
}

func (r *Raft) process(entry raftpb.Entry) {
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		var meta ObjectMeta
		err := proto.Unmarshal(entry.Data, &meta)
		if err != nil {
			logger.Warningf("alaya raft process object meta in entry err: %v", err)
		}
		logger.Tracef("node %v, PG: %v, New object meta: %v", r.raftCfg.ID, r.pgID, meta.ObjId)
		err = r.metaStorage.RecordMeta(&meta)
		if err != nil {
			logger.Warningf("alaya record object meta err: %v", err)
		}
	}
}

func (r *Raft) ProposeObjectMeta(meta *ObjectMeta) {
	bytes, _ := proto.Marshal(meta)
	err := r.raft.Propose(r.ctx, bytes)
	if err != nil {
		logger.Warningf("raft propose err: %v", err)
	}
}