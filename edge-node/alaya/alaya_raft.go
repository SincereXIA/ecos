package alaya

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/pipeline"
	eraft "ecos/edge-node/raft-node"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/logger"
	mapset "github.com/deckarep/golang-set"
	"github.com/gogo/protobuf/proto"
	"github.com/rcrowley/go-metrics"
	"github.com/wxnacy/wgo/arrays"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"path"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type Raft struct {
	ctx    context.Context //context
	cancel context.CancelFunc

	pgID uint64

	watcher *watcher.Watcher

	raft        *eraft.RaftNode
	snapshotter *snap.Snapshotter

	raftAlayaChan chan raftpb.Message
	stopChan      chan uint64

	metaStorage   MetaStorage
	metaApplyChan chan *MetaOperate

	pipeline *pipeline.Pipeline
	// rwMutex protect pipeline
	rwMutex sync.RWMutex

	confChangeChan chan raftpb.ConfChange
}

func NewAlayaRaft(raftID uint64, nowPipe *pipeline.Pipeline, oldP *pipeline.Pipeline,
	watcher *watcher.Watcher, metaStorage MetaStorage,
	raftAlayaChan chan raftpb.Message, stopChan chan uint64) *Raft {

	ctx, cancel := context.WithCancel(context.Background())

	r := &Raft{
		pgID:          nowPipe.PgId,
		ctx:           ctx,
		cancel:        cancel,
		watcher:       watcher,
		raftAlayaChan: raftAlayaChan,
		metaStorage:   metaStorage,

		pipeline:       nowPipe,
		stopChan:       stopChan,
		confChangeChan: make(chan raftpb.ConfChange, 100), // TODO: not ok
		metaApplyChan:  make(chan *MetaOperate, 100),
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
	logger.Infof("%v new raft node for pg %v, peers: %v, oldP: %v", raftID, r.pgID, peers, oldP)

	// TODO: maybe readyC is useless
	readyC := make(chan bool)
	// TODO: init wal base path
	basePath := path.Join("ecos-data/alaya/", "pg"+pgIdToStr(r.pgID), strconv.FormatInt(int64(raftID), 10))
	snapshotterReady, raftNode := eraft.NewRaftNode(int(raftID), r.ctx, peers, basePath, readyC, r.metaStorage.CreateSnapshot)
	r.raft = raftNode
	<-readyC
	r.snapshotter = <-snapshotterReady

	return r
}

func (r *Raft) cleanup() {
	logger.Warningf("raft %d stopped, start cleanup", r.raft.ID)
	// TODO: close cf?
	logger.Warningf("moon %d clean up done", r.raft.ID)
}

func (r *Raft) Run() {
	go r.RunAskForLeader()
	go r.readCommit(r.raft.CommitC, r.raft.ErrorC)

	for {
		select {
		case <-r.ctx.Done():
			r.cleanup()
			return
		case msgs := <-r.raft.CommunicationC:
			go r.sendMsgByRpc(msgs)
		case message := <-r.raftAlayaChan:
			//logger.Infof("%v send message %v to etcd raft", r.raft.ID, message)
			r.raft.RaftChan <- message
			//logger.Infof("%v send message %v to etcd raft success", r.raft.ID, message)
		case cc := <-r.raft.ApplyConfChangeC:
			//logger.Infof("%v apply conf change %v", r.raft.ID, cc)
			r.CheckConfChange(&cc)
		}
	}
}

func (r *Raft) loadSnapshot() (*raftpb.Snapshot, error) {
	snapshot, err := r.snapshotter.Load()
	if err == snap.ErrNoSnapshot {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (r *Raft) readCommit(commitC <-chan *eraft.Commit, errorC <-chan error) {
	for commit := range commitC {
		if commit == nil {
			snapshot, err := r.loadSnapshot()
			if err != nil {
				logger.Errorf("failed to load snapshot: %v", err)
			}
			if snapshot != nil {
				logger.Infof("%d loading snapshot at term %d and index %d", r.raft.ID, snapshot.Metadata.Term, snapshot.Metadata.Index)
				if err := r.metaStorage.RecoverFromSnapshot(snapshot.Data); err != nil {
					logger.Errorf("[%v] failed to recover from snapshot: %v", r.raft.ID, err)
				} else {
					logger.Infof("[%v] recover from snapshot success", r.raft.ID)
				}
			}
			continue
		}

		for _, rawData := range commit.Data {
			var metaOperate MetaOperate
			data := []byte(rawData)
			err := metaOperate.Unmarshal(data)
			if err != nil {
				logger.Warningf("alaya raft process object meta in entry err: %v", err)
			}
			switch metaOperate.Operate {
			case MetaOperate_PUT:
				meta := metaOperate.Meta
				logger.Infof("node %v, PG: %v, New object meta: %v", r.raft.ID, r.pgID, meta.ObjId)
				err = r.metaStorage.RecordMeta(meta)
				if err != nil {
					logger.Warningf("alaya record object meta err: %v", err)
				}
				metrics.GetOrRegisterCounter(watcher.MetricsAlayaMetaCount, nil).Inc(1)
			case MetaOperate_DELETE:
				logger.Infof("delete meta: %v", metaOperate.Meta.ObjId)
				err = r.metaStorage.Delete(metaOperate.Meta.ObjId)
				metrics.GetOrRegisterCounter(watcher.MetricsAlayaMetaCount, nil).Dec(1)
			default:
				logger.Errorf("unsupported alaya meta operate")
			}
			if err != nil {
				logger.Errorf("Alaya process Alaya message err: %v", err.Error())
			}
			r.metaApplyChan <- &metaOperate
		}
		close(commit.ApplyDoneC) // TODO: (qiu) what use for
	}
	if err, ok := <-errorC; ok {
		logger.Fatalf("commit stream error: %v", err)
	}
}

func (r *Raft) CheckConfChange(change *raftpb.ConfChange) {
	if len(change.Context) > 0 {
		if r.raft.Node.Status().Lead == r.raft.Node.Status().ID {
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
	if change.NodeID != uint64(r.raft.ID) {
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
	for {
		select {
		case <-r.confChangeChan:
		default:
			goto START
		}
	}
START:
	go func() {
		needAdd, needRemove := calDiff(newP.RaftId, oldP.RaftId)
		err := r.ProposeNewNodes(needAdd)
		if err != nil {
			logger.Errorf("Alaya propose new nodes in PG: %v fail, err: %v", r.pgID, err)
		}
		if r.raft.Node.Status().ID != r.getPipeline().RaftId[0] {
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
RETRY_ADD:
	for _, id := range NodeIDs {
		logger.Infof("raft: %v PG: %v propose conf change addNode: %v", r.raft.ID, r.pgID, id)
		data, _ := r.getPipeline().Marshal()
		r.raft.Node.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeAddNode,
			NodeID:  id,
			Context: data,
		})
		for {
			select {
			case <-time.After(time.Duration(time.Second * 5)):
				logger.Infof("raft %v retry to add nodes", r.raft.ID)
				goto RETRY_ADD
			case change := <-r.confChangeChan:
				if change.NodeID != id {
					r.confChangeChan <- change
					runtime.Gosched()
				} else {
					logger.Infof("raft: %v PG: %v success add node: %v", r.raft.ID, r.pgID, id)
					return nil
				}
			}
		}
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
		if id == r.raft.Node.Status().ID {
			removeSelf = true
			continue
		}
	RETRY_REMOVE:
		logger.Infof("raft: %v PG: %v propose conf change removeNode: %v", r.raft.ID, r.pgID, id)
		err := r.raft.Node.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeRemoveNode,
			NodeID:  id,
			Context: data,
		})
		for err != nil {
			err = r.raft.Node.ProposeConfChange(r.ctx, raftpb.ConfChange{
				Type:    raftpb.ConfChangeAddNode,
				NodeID:  id,
				Context: data,
			})
			runtime.Gosched()
		}
		// 检查是否 propose 了本次节点
		flag := false
		for {
			select {
			case <-time.After(time.Duration(time.Second * 2)):
				logger.Infof("raft %v retry to remove node", r.raft.ID)
				goto RETRY_REMOVE
			case change := <-r.confChangeChan:
				if change.NodeID != id {
					r.confChangeChan <- change
					runtime.Gosched()
				} else {
					logger.Infof("raft: %v PG: %v success remove node: %v", r.raft.ID, r.pgID, id)
					flag = true
					break
				}
			}
			if flag {
				break
			}
		}
	}
	if removeSelf {
		logger.Infof("raft: %v PG: %v propose conf change remove self", r.raft.ID, r.pgID)
		_ = r.raft.Node.ProposeConfChange(r.ctx, raftpb.ConfChange{
			Type:    raftpb.ConfChangeRemoveNode,
			NodeID:  uint64(r.raft.ID),
			Context: data,
		})
		//<-r.confChangeChan
	}
	return nil
}

func (r *Raft) Stop() {
	logger.Infof("=========STOP: node: %v, PG: %v ===========", r.raft.ID, r.pgID)
	r.stopChan <- r.pgID
	r.cancel()
	logger.Infof("=========STOP: node: %v, PG: %v done ===========", r.raft.ID, r.pgID)
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
	//logger.Infof("%v sendMsgByRpc: %v", r.raft.ID, messages)
	for _, message := range messages {
		select {
		case <-r.ctx.Done():
			return
		default:
		}
		if message.Type == raftpb.MsgSnap {
			message.Snapshot.Metadata.ConfState = r.raft.ConfState
		}
		//logger.Debugf("%v send msg to node: %v, msg: %v", r.raft.ID, message.To, message)
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

// ProposeObjectMetaOperate Propose a request to operate object meta to raft group,
// and wait it applied into meta storage
func (r *Raft) ProposeObjectMetaOperate(operate *MetaOperate) error {
	bytes, _ := proto.Marshal(operate)
	err := r.raft.Node.Propose(r.ctx, bytes)
	if err != nil {
		logger.Warningf("raft propose err: %v", err)
		return err
	}
	// TODO (zhang): Time out
	for {
		m := <-r.metaApplyChan
		if operate.Operate != m.Operate || m.Meta.ObjId != operate.Meta.ObjId {
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
			logger.Debugf("PG: %v, node%v Stop askForLeader", r.pgID, r.raft.ID)
			return
		default:
		}
		time.Sleep(1 * time.Second)
		if r.raft == nil || r.raft.Node.Status().Lead == uint64(0) || r.raft.Node.Status().Lead == uint64(r.raft.ID) ||
			uint64(r.raft.ID) != r.getPipeline().RaftId[0] || !r.pipelineReady() {
			continue
		} else {
			r.askForLeader()
			needRemove, _ := calDiff(r.GetVotersID(), r.getPipeline().RaftId)
			if len(needRemove) > 0 {
				logger.Infof("raft: %v, PG: %v Start remove nodes: %v", r.raft.ID, r.pgID, needRemove)
				err := r.ProposeRemoveNodes(needRemove)
				if err != nil {
					logger.Errorf("Alaya propose remove nodes in PG: %v fail, err: %v", r.pgID, err)
				}
			}
		}
	}
}

func (r *Raft) askForLeader() {
	logger.Infof("PG: %v, node%v askForLeader", r.pgID, r.raft.ID)
	r.raft.Node.TransferLeadership(r.ctx, r.raft.Node.Status().Lead, r.raft.Node.Status().ID)
	for {
		if r.isLeader() {
			return
		}
		logger.Infof("PG: %v, node %v not leader", r.pgID, r.raft.ID)
		leader := r.raft.Node.Status().Lead
		r.raft.Node.TransferLeadership(r.ctx, leader, r.raft.Node.Status().ID)
		time.Sleep(time.Second)
	}
}

func (r *Raft) GetVotersID() (rs []uint64) {
	for id := range r.raft.Node.Status().Config.Voters.IDs() {
		rs = append(rs, id)
	}
	return rs
}

func (r *Raft) isLeader() bool {
	return r.raft.Node.Status().Lead == uint64(r.raft.ID)
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
