package alaya

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/logger"
	mapset "github.com/deckarep/golang-set"
	"github.com/gogo/protobuf/proto"
	"github.com/wxnacy/wgo/arrays"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"strconv"
	"sync"
	"time"
)

type Raft struct {
	pgID   uint64
	ctx    context.Context //context
	cancel context.CancelFunc

	watcher     *watcher.Watcher
	raftStorage *raft.MemoryStorage //raft需要的内存结构
	raftCfg     *raft.Config        //raft需要的配置
	raft        raft.Node
	ticker      <-chan time.Time //定时器，提供周期时钟源和超时触发能力

	raftChan chan raftpb.Message
	stopChan chan uint64

	metaStorage   MetaStorage
	metaApplyChan chan object.ObjectMeta

	pipeline *pipeline.Pipeline
	// rwMutex protect pipeline
	rwMutex sync.RWMutex

	confChangeChan chan raftpb.ConfChange
}

func NewAlayaRaft(raftID uint64, nowPipe *pipeline.Pipeline, oldP *pipeline.Pipeline,
	watcher *watcher.Watcher, metaStorage MetaStorage,
	raftChan chan raftpb.Message, stopChan chan uint64) *Raft {

	ctx, cancel := context.WithCancel(context.Background())
	raftStorage := raft.NewMemoryStorage()
	ticker := time.NewTicker(time.Millisecond * 300)

	r := &Raft{
		pgID:        nowPipe.PgId,
		ctx:         ctx,
		cancel:      cancel,
		watcher:     watcher,
		raftStorage: raftStorage,
		raftCfg: &raft.Config{
			ID:              raftID,
			ElectionTick:    10,
			HeartbeatTick:   1,
			Storage:         raftStorage,
			MaxSizePerMsg:   1024 * 1024,
			MaxInflightMsgs: 256,
		},
		ticker:      ticker.C,
		raftChan:    raftChan,
		metaStorage: metaStorage,

		pipeline:       nowPipe,
		stopChan:       stopChan,
		confChangeChan: make(chan raftpb.ConfChange, 100), // TODO: not ok
		metaApplyChan:  make(chan object.ObjectMeta, 100),
	}

	var peers []raft.Peer
	if oldP != nil {
		for _, id := range oldP.RaftId {
			peers = append(peers, raft.Peer{
				ID: id,
			})
		}
	} else {
		for _, id := range nowPipe.RaftId {
			peers = append(peers, raft.Peer{
				ID: id,
			})
		}
	}

	r.raft = raft.StartNode(r.raftCfg, peers)
	return r
}

func (r *Raft) Run() {
	go r.RunAskForLeader()
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
	if len(change.Context) > 0 {
		if r.raft.Status().Lead == r.raft.Status().ID {
			// this message sent by propose new pipeline
			r.confChangeChan <- *change
		} else {
			var p pipeline.Pipeline
			err := p.Unmarshal(change.Context)
			if err != nil {
				logger.Errorf("get pipeline from conf change fail")
			}
			r.setPipeline(&p)
		}
	}
	if change.NodeID != r.raftCfg.ID {
		return
	}
	if change.Type == raftpb.ConfChangeRemoveNode {
		go func() {
			r.Stop()
		}()
	}
}

func (r *Raft) ProposeNewPipeline(newP *pipeline.Pipeline, oldP *pipeline.Pipeline) {
	r.setPipeline(newP)
	// clear confChangeChan, to wait conf change propose
	select {
	case <-r.confChangeChan:
	default:
	}
	go func() {
		needAdd, needRemove := calDiff(newP.RaftId, oldP.RaftId)
		err := r.ProposeNewNodes(needAdd)
		if err != nil {
			logger.Errorf("Alaya propose new nodes in PG: %v fail, err: %v", r.pgID, err)
		}
		if r.raft.Status().ID != r.getPipeline().RaftId[0] {
			return
		}
		err = r.ProposeRemoveNodes(needRemove)
		if err != nil {
			logger.Errorf("Alaya propose remove nodes in PG: %v fail, err: %v", r.pgID, err)
		}
	}()
}

func (r *Raft) ProposeNewNodes(NodeIDs []uint64) error {
	if len(NodeIDs) == 0 {
		return nil
	}
	for _, id := range NodeIDs {
		logger.Infof("raft: %v PG: %v propose conf change addNode: %v", r.raftCfg.ID, r.pgID, id)
		data, _ := r.getPipeline().Marshal()
		_ = r.raft.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  id,
			Context: data,
		})
		<-r.confChangeChan
	}
	return nil
}

func (r *Raft) ProposeRemoveNodes(NodeIDs []uint64) error {
	select {
	case <-r.ctx.Done():
		return nil
	default:
	}
	if len(NodeIDs) == 0 {
		return nil
	}
	removeSelf := false
	data, _ := r.getPipeline().Marshal()
	for _, id := range NodeIDs {
		if id == r.raft.Status().ID {
			removeSelf = true
			continue
		}
		logger.Infof("raft: %v PG: %v propose conf change removeNode: %v", r.raftCfg.ID, r.pgID, id)
		_ = r.raft.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeRemoveNode,
			NodeID:  id,
			Context: data,
		})
		<-r.confChangeChan
	}
	if removeSelf {
		logger.Infof("raft: %v PG: %v propose conf change remove self", r.raftCfg.ID, r.pgID)
		_ = r.raft.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeRemoveNode,
			NodeID:  r.raft.Status().ID,
			Context: data,
		})
		<-r.confChangeChan
	}
	return nil
}

func (r *Raft) Stop() {
	logger.Infof("=========STOP: node: %v, PG: %v ===========", r.raft.Status().ID, r.pgID)
	r.cancel()
	r.stopChan <- r.pgID
	logger.Infof("=========STOP: node: %v, PG: %v done ===========", r.raft.Status().ID, r.pgID)
}

func (r *Raft) getNodeInfo(nodeID uint64) (*infos.NodeInfo, error) {
	req := &moon.GetInfoRequest{
		InfoId:   strconv.FormatUint(nodeID, 10),
		InfoType: infos.InfoType_NODE_INFO,
	}
	result, err := r.watcher.GetMoon().GetInfo(r.ctx, req)
	if err != nil {
		return nil, err
	}
	return result.BaseInfo.GetNodeInfo(), nil
}

func (r *Raft) sendMsgByRpc(messages []raftpb.Message) {
	for _, message := range messages {
		nodeId := message.To
		nodeInfo, err := r.getNodeInfo(nodeId)
		if err != nil {
			logger.Errorf("Get nodeInfo: %v fail: %v", nodeId, err)
			return
		}
		conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
		if err != nil {
			logger.Warningf("faild to connect: %v", err)
			continue
		}
		// TODO: 缓存 rpc 连接后这里不用关闭
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
		var metaOperate MetaOperate
		err := proto.Unmarshal(entry.Data, &metaOperate)
		if err != nil {
			logger.Warningf("alaya raft process object meta in entry err: %v", err)
		}
		switch metaOperate.Operate {
		case MetaOperate_PUT:
			meta := metaOperate.Meta
			logger.Infof("node %v, PG: %v, New object meta: %v", r.raftCfg.ID, r.pgID, meta.ObjId)
			err = r.metaStorage.RecordMeta(meta)
			if err != nil {
				logger.Warningf("alaya record object meta err: %v", err)
			}
		case MetaOperate_DELETE:
			err = r.metaStorage.Delete(metaOperate.Meta.ObjId)
			logger.Errorf("DELETE Meta not implement")
		}
		r.metaApplyChan <- *metaOperate.Meta
	}
}

// ProposeObjectMetaOperate Propose a request to operate object meta to raft group,
// and wait it applied into meta storage
func (r *Raft) ProposeObjectMetaOperate(operate *MetaOperate) error {
	bytes, _ := proto.Marshal(operate)
	err := r.raft.Propose(r.ctx, bytes)
	if err != nil {
		logger.Warningf("raft propose err: %v", err)
		return err
	}
	// TODO (zhang): Time out
	for {
		m := <-r.metaApplyChan
		if m.ObjId != operate.Meta.ObjId {
			r.metaApplyChan <- m
		} else {
			return nil
		}
	}
}

func (r *Raft) RunAskForLeader() {
	for {
		select {
		case <-r.ctx.Done():
			logger.Debugf("PG: %v, node%v Stop askForLeader", r.pgID, r.raftCfg.ID)
			return
		default:
		}
		time.Sleep(1 * time.Second)
		if r.raft == nil || r.raft.Status().Lead == uint64(0) || r.raft.Status().Lead == r.raftCfg.ID ||
			r.raftCfg.ID != r.getPipeline().RaftId[0] || !r.pipelineReady() {
			continue
		} else {
			r.askForLeader()
			needRemove, _ := calDiff(r.GetVotersID(), r.getPipeline().RaftId)
			if len(needRemove) > 0 {
				logger.Infof("raft: %v, PG: %v Start remove nodes: %v", r.raft.Status().ID, r.pgID, needRemove)
				err := r.ProposeRemoveNodes(needRemove)
				if err != nil {
					logger.Errorf("Alaya propose remove nodes in PG: %v fail, err: %v", r.pgID, err)
				}
			}
		}
	}
}

func (r *Raft) askForLeader() {
	logger.Infof("PG: %v, node%v askForLeader", r.pgID, r.raftCfg.ID)
	r.raft.TransferLeadership(r.ctx, r.raft.Status().Lead, r.raft.Status().ID)
	for {
		if r.isLeader() {
			return
		}
		logger.Infof("PG: %v, node %v not leader", r.pgID, r.raftCfg.ID)
		r.raft.TransferLeadership(r.ctx, r.raft.Status().Lead, r.raft.Status().ID)
		time.Sleep(time.Second)
	}
}

func (r *Raft) GetVotersID() (rs []uint64) {
	for id := range r.raft.Status().Config.Voters.IDs() {
		rs = append(rs, id)
	}
	return rs
}

func (r *Raft) isLeader() bool {
	return r.raft.Status().Lead == r.raft.Status().ID
}

func (r *Raft) pipelineReady() bool {
	r.rwMutex.RLock()
	defer r.rwMutex.RUnlock()
	voters := r.GetVotersID()
	for _, id := range r.pipeline.RaftId {
		if -1 == arrays.Contains(voters, id) {
			logger.Infof("not ready, not have: %v", id)
			return false
		}
	}
	return true
}

func (r *Raft) getPipeline() *pipeline.Pipeline {
	r.rwMutex.RLock()
	defer r.rwMutex.RUnlock()
	p := &r.pipeline
	return *p
}

func (r *Raft) setPipeline(p *pipeline.Pipeline) {
	r.rwMutex.Lock()
	defer r.rwMutex.Unlock()
	r.pipeline = p
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
