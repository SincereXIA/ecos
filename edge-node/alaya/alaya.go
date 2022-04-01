package alaya

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
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

	selfInfo       *infos.NodeInfo
	watcher        *watcher.Watcher
	PGMessageChans sync.Map
	// PGRaftNode save all raft node on this edge.
	// pgID -> Raft node
	PGRaftNode sync.Map
	// mutex ensure Alaya.Run after pipeline ready
	mutex            sync.Mutex
	raftNodeStopChan chan uint64 // 内部 raft node 主动退出时，使用该 chan 通知 alaya

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
	// check if meta belongs to this PG
	_, bucketID, key, err := object.SplitID(meta.ObjId)
	if err != nil {
		return nil, err
	}
	m := a.watcher.GetMoon()
	info, err := m.GetInfoDirect(infos.InfoType_BUCKET_INFO, bucketID)
	if err != nil {
		return nil, err
	}
	bucketInfo := info.BaseInfo().GetBucketInfo()
	pgID := object.GenObjPgID(bucketInfo, key, 10)
	if meta.Term != a.watcher.GetCurrentTerm() {
		return nil, errno.TermNotMatch
	}
	if meta.PgId != pgID {
		logger.Warningf("meta pgID %d not match calculated pgID %d", meta.PgId, pgID)
		return nil, errno.PgNotMatch
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		raft := a.getRaftNode(pgID)
		if raft == nil {
			return nil, errno.RaftNodeNotFound
		}
		// 这里是同步操作
		err = a.getRaftNode(pgID).ProposeObjectMetaOperate(&MetaOperate{
			Operate: MetaOperate_PUT,
			Meta:    meta,
		})
		if err != nil {
			return &common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			}, err
		}
		logger.Infof("Alaya record object meta success, obj_id: %v, size: %v", meta.ObjId, meta.ObjSize)
	}

	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (a *Alaya) GetObjectMeta(ctx context.Context, req *MetaRequest) (*object.ObjectMeta, error) {
	objId := req.ObjId
	objMeta, err := a.MetaStorage.GetMeta(objId)
	if err != nil {
		logger.Errorf("alaya get metaStorage by objID failed, err: %v", err)
		return nil, err
	}
	return objMeta, nil
}

func (a *Alaya) SendRaftMessage(ctx context.Context, pgMessage *PGRaftMessage) (*PGRaftMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
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
	a.cancel()
}

func (a *Alaya) cleanup() {
	logger.Warningf("alaya %v stopped, start cleanup", a.selfInfo.RaftId)
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		go value.(*Raft).Stop()
		<-a.raftNodeStopChan
		return true
	})
	a.MetaStorage.Close()
}

func (a *Alaya) applyNewClusterInfo(info infos.Information) {
	a.ApplyNewClusterInfo(info.BaseInfo().GetClusterInfo())
}

func (a *Alaya) ApplyNewClusterInfo(clusterInfo *infos.ClusterInfo) {
	// TODO: (zhang) make pgNum configurable
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.state = UPDATING
	oldPipelines := a.pipelines
	if clusterInfo == nil || len(clusterInfo.NodesInfo) == 0 {
		logger.Warningf("Empty clusterInfo when alaya apply new clusterInfo")
		return
	}
	logger.Infof("Alaya: %v receive new cluster info, term: %v, node num: %v", a.selfInfo.RaftId,
		clusterInfo.Term, len(clusterInfo.NodesInfo))
	p := pipeline.GenPipelines(*clusterInfo, 10, 3)
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

		if raftNode.raft.Status().Lead != a.selfInfo.RaftId {
			return true
		}
		// if this node is leader, add new pipelines node first
		p := pipelines[pgID-1]
		raftNode.ProposeNewPipeline(p, oldPipelines[pgID-1])
		return true
	})

	// start run raft node new in pipelines
	for _, p := range pipelines {
		if -1 == arrays.Contains(p.RaftId, a.selfInfo.RaftId) { // pass when node not in pipeline
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

func NewAlaya(ctx context.Context, watcher *watcher.Watcher,
	metaStorage MetaStorage, rpcServer *messenger.RpcServer) *Alaya {
	ctx, cancel := context.WithCancel(ctx)
	a := Alaya{
		ctx:              ctx,
		cancel:           cancel,
		PGMessageChans:   sync.Map{},
		PGRaftNode:       sync.Map{},
		selfInfo:         watcher.GetSelfInfo(),
		MetaStorage:      metaStorage,
		state:            INIT,
		watcher:          watcher,
		raftNodeStopChan: make(chan uint64),
	}
	RegisterAlayaServer(rpcServer, &a)
	clusterInfo := watcher.GetCurrentClusterInfo()
	a.ApplyNewClusterInfo(&clusterInfo)
	a.state = READY
	_ = a.watcher.SetOnInfoUpdate(infos.InfoType_CLUSTER_INFO, "moon", a.applyNewClusterInfo)

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
			a.cleanup()
			return
		}
	}
}

func (a *Alaya) PrintPipelineInfo() {
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		raftNode := value.(*Raft)
		pgID := key.(uint64)
		logger.Infof("Alaya: %v, PG: %v, leader: %v, voter: %v", a.selfInfo.RaftId, pgID, raftNode.raft.Status().Lead, raftNode.GetVotersID())

		return true
	})
}

// IsAllPipelinesOK check if pipelines in THIS alaya node is ok.
// each raftNode should have leader, and each group should have exactly 3 nodes.
func (a *Alaya) IsAllPipelinesOK() bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	ok := true
	length := 0
	if len(a.pipelines) == 0 || a.state != RUNNING {
		return false
	}
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		raftNode := value.(*Raft)
		if raftNode.raft.Status().Lead != raftNode.getPipeline().RaftId[0] ||
			len(raftNode.GetVotersID()) != len(raftNode.getPipeline().RaftId) {
			ok = false
			return false
		}
		length += 1
		return true
	})
	return ok
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
		NewAlayaRaft(a.selfInfo.RaftId, p, oldP, a.watcher, a.MetaStorage,
			c.(chan raftpb.Message), a.raftNodeStopChan))
	logger.Infof("Node: %v successful add raft node in alaya, PG: %v", a.selfInfo.RaftId, pgID)
	return a.getRaftNode(pgID)
}
