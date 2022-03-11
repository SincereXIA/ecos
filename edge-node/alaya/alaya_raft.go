package alaya

import (
	"context"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/utils/logger"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"strconv"
	"time"
)

type Raft struct {
	pgID        uint64
	ctx         context.Context //context
	cancel      context.CancelFunc
	InfoStorage node.InfoStorage
	raftStorage *raft.MemoryStorage //raft需要的内存结构
	raftCfg     *raft.Config        //raft需要的配置
	raft        raft.Node
	ticker      <-chan time.Time //定时器，提供周期时钟源和超时触发能力

	raftChan chan raftpb.Message

	metaStorage MetaStorage

	pipeline    *pipeline.Pipeline
	controlChan chan ControlMsg
}

type ControlMsg int

const (
	AddNodeFinish ControlMsg = iota
)

func NewAlayaRaft(raftID uint64, pgID uint64, nowPipe *pipeline.Pipeline, oldP *pipeline.Pipeline,
	infoStorage node.InfoStorage, metaStorage MetaStorage,
	raftChan chan raftpb.Message) *Raft {

	ctx, cancel := context.WithCancel(context.Background())
	raftStorage := raft.NewMemoryStorage()
	ticker := time.NewTicker(time.Millisecond * 100)

	r := &Raft{
		pgID:        pgID,
		ctx:         ctx,
		cancel:      cancel,
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
		ticker:      ticker.C,
		raftChan:    raftChan,
		metaStorage: metaStorage,

		controlChan: make(chan ControlMsg),
		pipeline:    nowPipe,
	}

	var peers []raft.Peer
	if oldP != nil {
		for _, id := range oldP.RaftId {
			peers = append(peers, raft.Peer{
				ID: id,
			})
		}

	}
	for _, id := range nowPipe.RaftId {
		peers = append(peers, raft.Peer{
			ID: id,
		})
	}

	r.raft = raft.StartNode(r.raftCfg, peers)
	return r
}

func (r *Raft) Run() {
	for {
		select {
		case <-r.ctx.Done():
			r.raft.Stop()
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
					r.CheckConfChange(&cc)
				}
			}
			r.raft.Advance()
		case message := <-r.raftChan:
			_ = r.raft.Step(r.ctx, message)
		}
	}
}

func (r *Raft) CheckConfChange(change *raftpb.ConfChange) {
	if change.NodeID != r.raftCfg.ID {
		return
	}
	if change.Type == raftpb.ConfChangeRemoveNode {
		r.Stop()
	}
}

func (r *Raft) ProposeNewNodes(NodeIDs []uint64) error {
	if len(NodeIDs) == 0 {
		return nil
	}
	for _, id := range NodeIDs {
		logger.Infof("raft: %v PG: %v propose conf change addNode: %v", r.raftCfg.ID, r.pgID, NodeIDs)
		_ = r.raft.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  id,
			Context: nil,
		})
		time.Sleep(time.Millisecond * 200)
	}
	// TODO: wait add new nodes ok
	r.controlChan <- AddNodeFinish
	return nil
}

func (r *Raft) ProposeRemoveNodes(NodeIDs []uint64) error {
	select {
	case <-r.ctx.Done():
		return nil
	case msg := <-r.controlChan:
		if msg == AddNodeFinish {
			break
		}
		r.controlChan <- msg
	}
	if len(NodeIDs) == 0 {
		return nil
	}
	time.Sleep(time.Second * 3)
	for _, id := range NodeIDs {
		logger.Infof("raft: %v PG: %v propose conf change removeNode: %v", r.raftCfg.ID, r.pgID, id)
		_ = r.raft.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeRemoveNode,
			NodeID:  id,
			Context: nil,
		})
		time.Sleep(time.Millisecond * 200)
	}
	return nil
}

func (r *Raft) Leave() error {
	err := r.raft.ProposeConfChange(r.ctx, raftpb.ConfChangeV2{
		Transition: raftpb.ConfChangeTransitionAuto,
		Changes: []raftpb.ConfChangeSingle{
			{
				Type:   raftpb.ConfChangeRemoveNode,
				NodeID: r.raftCfg.ID,
			},
		},
		Context: nil,
	})
	if err != nil {
		return err
	}
	select {
	case <-r.ctx.Done():
		return nil
	}
}

func (r *Raft) Stop() {
	logger.Infof("=========STOP: node: %v, PG: %v ===========", r.raft.Status().ID, r.pgID)
	r.cancel()
}

func (r *Raft) sendMsgByRpc(messages []raftpb.Message) {
	for _, message := range messages {
		nodeId := node.ID(message.To)
		nodeInfo, err := r.InfoStorage.GetNodeInfo(nodeId)
		if err != nil {
			logger.Errorf("Get nodeInfo: %v fail: %v", nodeId, err)
			return
		}
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
		_, err = c.SendRaftMessage(context.TODO(), &PGRaftMessage{ // 这里不用当前 ctx 发送，否则当节点停止之后，最后的确认信息无法发送
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
		var meta object.ObjectMeta
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

func (r *Raft) ProposeObjectMeta(meta *object.ObjectMeta) {
	bytes, _ := proto.Marshal(meta)
	err := r.raft.Propose(r.ctx, bytes)
	if err != nil {
		logger.Warningf("raft propose err: %v", err)
	}
}

func (r *Raft) RunAskForLeader() {
	for {
		select {
		case <-r.ctx.Done():
			logger.Infof("PG: %v, node%v Stop askForLeader", r.pgID, r.raftCfg.ID)
			return
		default:
		}
		time.Sleep(1 * time.Second)
		if r.raft == nil || r.raft.Status().Lead == uint64(0) || r.raft.Status().Lead == r.raftCfg.ID {
			continue
		} else {
			logger.Infof("PG: %v, node%v askForLeader", r.pgID, r.raftCfg.ID)
			msg := []raftpb.Message{
				{
					From: r.raftCfg.ID,
					To:   r.raft.Status().Lead,
					Type: raftpb.MsgTransferLeader,
				},
			}
			r.sendMsgByRpc(msg)
		}
	}
}
