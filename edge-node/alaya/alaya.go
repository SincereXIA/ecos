package alaya

import (
	"context"
	"ecos/edge-node/cleaner"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/alaya"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"github.com/rcrowley/go-metrics"
	"github.com/wxnacy/wgo/arrays"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"sync"
	"time"
)

type Alayaer interface {
	alaya.AlayaServer
	Run()
	Stop()
	IsAllPipelinesOK() bool
	watcher.Reporter
}

// Alaya process record & inquire object Mata request
// 阿赖耶处理对象元数据的存储和查询请求
// 一切众生阿赖耶识，本来而有圆满清净，出过于世同于涅槃
type Alaya struct {
	alaya.UnimplementedAlayaServer

	config *Config

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

	MetaStorageRegister MetaStorageRegister
	cleaner             *cleaner.Cleaner

	state State

	clusterPipelines *pipeline.ClusterPipelines
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

func (a *Alaya) calculateObjectPGID(term uint64, objectID string) uint64 {
	_, bucketID, key, _, err := object.SplitID(objectID)
	if err != nil {
		return 0
	}
	m := a.watcher.GetMoon()
	info, err := m.GetInfoDirect(infos.InfoType_BUCKET_INFO, bucketID)
	if err != nil {
		return 0
	}
	bucketInfo := info.BaseInfo().GetBucketInfo()
	clusterInfo, err := a.watcher.GetClusterInfoByTerm(term)
	if err != nil {
		return 0
	}
	pgID := object.GenObjPgID(bucketInfo, key, clusterInfo.MetaPgNum)
	return pgID
}

func (a *Alaya) checkClientTerm(ctx context.Context) (uint64, error) {
	clientTerm, err := alaya.GetTermFromContext(ctx)
	if err != nil {
		return 0, err
	}
	if clientTerm != a.watcher.GetCurrentTerm() {
		logger.Warningf("client term %d not equal current term %d", clientTerm, a.watcher.GetCurrentTerm())
		return 0, errno.TermNotMatch
	}
	return clientTerm, nil
}

func (a *Alaya) checkObject(term uint64, meta *object.ObjectMeta) (err error) {
	// clear meta object id
	meta.ObjId = object.CleanObjectKey(meta.ObjId)

	// check if meta belongs to this PG
	pgID := a.calculateObjectPGID(term, meta.ObjId)
	if meta.PgId != pgID {
		logger.Warningf("meta pgID %d not match calculated pgID %d", meta.PgId, pgID)
		return errno.PgNotMatch
	}
	return nil
}

// RecordObjectMeta record object meta to MetaStorage
// 将对象元数据存储到 MetaStorage
func (a *Alaya) RecordObjectMeta(ctx context.Context, meta *object.ObjectMeta) (*common.Result, error) {
	timeStart := time.Now()
	term, err := a.checkClientTerm(ctx)
	if err != nil {
		return nil, err
	}
	err = a.checkObject(term, meta)
	if err != nil {
		return nil, err
	}
	pgID := meta.PgId
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		raft := a.getRaftNode(pgID)
		if raft == nil {
			return nil, errno.RaftNodeNotFound
		}
		// 这里是同步操作
		err = a.getRaftNode(pgID).ProposeObjectMetaOperate(&alaya.MetaOperate{
			Operate: alaya.MetaOperate_PUT,
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
	metrics.GetOrRegisterTimer(watcher.MetricsAlayaMetaPutTimer, nil).UpdateSince(timeStart)

	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (a *Alaya) GetObjectMeta(ctx context.Context, req *alaya.MetaRequest) (*object.ObjectMeta, error) {
	// clear meta object id
	timeStart := time.Now()
	term, err := a.checkClientTerm(ctx)
	if err != nil {
		return nil, err
	}
	objID := object.CleanObjectKey(req.ObjId)
	pgID := a.calculateObjectPGID(term, objID)
	storage, err := a.MetaStorageRegister.GetStorage(pgID)
	if err != nil {
		return nil, err
	}
	objMeta, err := storage.GetMeta(objID)
	if err == errno.MetaNotExist {
		logger.Warningf("alaya receive get meta request, but meta not exist, obj_id: %v", objID)
		return nil, err
	}
	if err != nil {
		logger.Errorf("alaya get metaStorage by objID failed, err: %v", err)
		return nil, err
	}
	metrics.GetOrRegisterTimer(watcher.MetricsAlayaMetaGetTimer, nil).UpdateSince(timeStart)
	return objMeta, nil
}

// DeleteMeta delete meta from metaStorage, and request delete object blocks
func (a *Alaya) DeleteMeta(ctx context.Context, req *alaya.DeleteMetaRequest) (*common.Result, error) {
	_, err := alaya.GetTermFromContext(ctx)
	if err != nil {
		return nil, err
	}
	objID := object.CleanObjectKey(req.ObjId)
	objMeta, err := a.GetObjectMeta(ctx, &alaya.MetaRequest{ObjId: objID})
	if err != nil {
		logger.Errorf("alaya get meta by objID failed, err: %v", err)
		return nil, err
	}
	pgID := objMeta.PgId
	logger.Infof("alaya receive delete meta request, obj_id: %v, pg_id: %v", objID, pgID)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		raft := a.getRaftNode(pgID)
		if raft == nil {
			return nil, errno.RaftNodeNotFound
		}
		// 这里是同步操作
		err = a.getRaftNode(pgID).ProposeObjectMetaOperate(&alaya.MetaOperate{
			Operate: alaya.MetaOperate_DELETE,
			Meta:    objMeta,
		})
		if err != nil {
			return &common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			}, err
		}
		go func() {
			err := a.cleaner.RemoveObjectBlocks(objMeta)
			if err != nil {
				logger.Errorf("remove object blocks fail, objID: %v err: %v", objMeta.ObjId, err.Error())
			}
		}()
		logger.Infof("Alaya delete object meta success, obj_id: %v", objMeta.ObjId)
	}

	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (a *Alaya) ListMeta(ctx context.Context, req *alaya.ListMetaRequest) (*alaya.ObjectMetaList, error) {
	_, err := alaya.GetTermFromContext(ctx)
	if err != nil {
		return nil, err
	}
	storages := a.MetaStorageRegister.GetAllStorage()
	var metasList [][]*object.ObjectMeta
	for _, storage := range storages {
		req.Prefix = object.CleanObjectKey(req.Prefix)
		metas, _ := storage.List(req.Prefix)
		metasList = append(metasList, metas)
	}
	if len(metasList) == 0 {
		return &alaya.ObjectMetaList{}, nil
	}
	size := 0
	for _, metas := range metasList {
		size += len(metas)
	}
	metas := make([]*object.ObjectMeta, 0, size)
	for _, meta := range metasList {
		metas = append(metas, meta...)
	}
	return &alaya.ObjectMetaList{
		Metas: metas,
	}, nil
}

func (a *Alaya) SendRaftMessage(ctx context.Context, pgMessage *alaya.PGRaftMessage) (*alaya.PGRaftMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	pgID := pgMessage.PgId

	if _, ok := a.PGRaftNode.Load(pgID); !ok {
		logger.Warningf("%v receive raft message from: %v, but pg: %v not exist", a.selfInfo.RaftId, pgMessage.Message, pgID)
		return nil, errno.PGNotExist
	}

	if msgChan, ok := a.PGMessageChans.Load(pgID); ok {
		msgChan.(chan raftpb.Message) <- *pgMessage.Message
		return &alaya.PGRaftMessage{
			PgId:    pgMessage.PgId,
			Message: &raftpb.Message{},
		}, nil
	}
	logger.Errorf("%v receive raft message from: %v, but pg: %v channel not exist", a.selfInfo.RaftId, pgMessage.GetMessage(), pgID)
	return nil, errno.PGNotExist
}

func (a *Alaya) Stop() {
	logger.Infof("Alaya %v stop", a.selfInfo.RaftId)
	a.cancel()
}

func (a *Alaya) cleanup() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	logger.Warningf("alaya %v stopped, start cleanup", a.selfInfo.RaftId)
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		go value.(*Raft).Stop()
		<-a.raftNodeStopChan
		return true
	})
	a.MetaStorageRegister.Close()
	a.state = STOPPED
	logger.Infof("alaya %v start cleanup done", a.selfInfo.RaftId)
}

func (a *Alaya) applyNewClusterInfo(info infos.Information) {
	a.ApplyNewClusterInfo(info.BaseInfo().GetClusterInfo())
}

func (a *Alaya) ApplyNewClusterInfo(clusterInfo *infos.ClusterInfo) {
	// TODO: (zhang) make pgNum configurable
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.state = UPDATING
	oldPipelines := a.clusterPipelines
	if clusterInfo.LastTerm != 0 {
		info, err := a.watcher.GetClusterInfoByTerm(clusterInfo.LastTerm)
		if err != nil {
			logger.Errorf("get cluster info by term fail, term: %v err: %v", clusterInfo.LastTerm, err.Error())
		}
		oldPipelines, _ = pipeline.NewClusterPipelines(info)
	}
	if clusterInfo == nil || len(clusterInfo.NodesInfo) == 0 {
		logger.Warningf("Empty clusterInfo when alaya apply new clusterInfo")
		return
	}
	logger.Infof("Alaya: %v receive new cluster info, term: %v, node num: %v", a.selfInfo.RaftId,
		clusterInfo.Term, len(clusterInfo.NodesInfo))
	p, err := pipeline.NewClusterPipelines(*clusterInfo)
	if err != nil {
		logger.Errorf("Alaya: %v create new cluster pipelines failed, err: %v", a.selfInfo.RaftId, err)
		return
	}
	a.ApplyNewPipelines(p, oldPipelines)
	a.state = RUNNING
}

// ApplyNewPipelines use new pipelines info to change raft nodes in Alaya
// pipelines must in order (from 1 to n)
func (a *Alaya) ApplyNewPipelines(pipelines *pipeline.ClusterPipelines, oldPipelines *pipeline.ClusterPipelines) {
	a.clusterPipelines = pipelines

	// Add raft node new in pipelines
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		raftNode := value.(*Raft)
		pgID := key.(uint64)
		p := pipelines.MetaPipelines[pgID-1]

		//if raftNode.raft.Node.Status().Lead != a.selfInfo.RaftId &&
		//	-1 == arrays.Contains(p.RaftId, raftNode.raft.Node.Status().Lead) {
		//	// 若当前节点不是 leader，并且原有 leader 仍然在新 pg 中，则当前 pg 可以跳过
		//	return true
		//}

		raftNode.ProposeNewPipeline(p)
		return true
	})

	// start run raft node new in pipelines
	for _, p := range pipelines.MetaPipelines {
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
			raft = a.makeAlayaRaftInPipeline(p, oldPipelines.MetaPipelines[pgID-1])
		} else {
			raft = a.makeAlayaRaftInPipeline(p, nil)
		}
		if a.state == UPDATING {
			go func(raft *Raft) {
				raft.Run()
			}(raft)
		}
	}
}

func NewAlaya(ctx context.Context, watcher *watcher.Watcher, config *Config,
	metaStorageRegister MetaStorageRegister, rpcServer *messenger.RpcServer) Alayaer {
	ctx, cancel := context.WithCancel(ctx)
	c := cleaner.NewCleaner(ctx, watcher)
	a := Alaya{
		config:              config,
		ctx:                 ctx,
		cancel:              cancel,
		PGMessageChans:      sync.Map{},
		PGRaftNode:          sync.Map{},
		selfInfo:            watcher.GetSelfInfo(),
		MetaStorageRegister: metaStorageRegister,
		state:               INIT,
		watcher:             watcher,
		cleaner:             c,
		raftNodeStopChan:    make(chan uint64),
	}
	alaya.RegisterAlayaServer(rpcServer, &a)
	clusterInfo := watcher.GetCurrentClusterInfo()
	a.ApplyNewClusterInfo(&clusterInfo)
	a.state = READY
	_ = a.watcher.SetOnInfoUpdate(infos.InfoType_CLUSTER_INFO,
		"alaya-"+a.watcher.GetSelfInfo().Uuid, a.applyNewClusterInfo)
	err := a.watcher.Monitor.Register("alaya", &a)
	errno.HandleError(err)
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
			metrics.GetOrRegisterCounter(watcher.MetricsAlayaPipelineCount, nil).Dec(1)
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
		logger.Infof("Alaya: %v, PG: %v, leader: %v, voter: %v",
			a.selfInfo.RaftId, pgID, raftNode.raft.Node.Status().Lead, raftNode.GetVotersID())
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
	if len(a.clusterPipelines.MetaPipelines) == 0 || a.state != RUNNING ||
		a.clusterPipelines.Term != a.watcher.GetCurrentTerm() {
		logger.Infof("term not ok")
		return false
	}
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		raftNode := value.(*Raft)
		if raftNode.raft.Node.Status().Lead != raftNode.getPipeline().RaftId[0] ||
			len(raftNode.GetVotersID()) != len(raftNode.getPipeline().RaftId) {
			ok = false
			return false
		}
		length += 1
		return true
	})
	return ok
}

func (a *Alaya) IsChanged() bool {
	return true
}

func (a *Alaya) GetReports() []watcher.Report {
	var reports []watcher.Report
	a.PGRaftNode.Range(func(key, value interface{}) bool {
		pgID := key.(uint64)
		reports = append(reports, a.getPipelineReport(pgID))
		return true
	})
	return reports
}

// reportPipeline 向 monitor 报告 pipeline 状态
func (a *Alaya) getPipelineReport(pgID uint64) watcher.Report {
	if value, ok := a.PGRaftNode.Load(pgID); ok {
		node := value.(*Raft)
		var state watcher.PipelineReport_State
		if node.raft.Node.Status().Lead == 0 {
			state = watcher.PipelineReport_ERROR
		} else if len(node.GetVotersID()) < len(node.getPipeline().RaftId) {
			state = watcher.PipelineReport_DOWN_GRADE
		} else if node.raft.Node.Status().Lead != node.getPipeline().RaftId[0] {
			state = watcher.PipelineReport_CHANGING
		} else {
			state = watcher.PipelineReport_OK
		}
		report := &watcher.PipelineReport{
			PgId:     pgID,
			NodeIds:  node.GetVotersID(),
			LeaderId: node.raft.Node.Status().Lead,
			State:    state,
		}
		return watcher.Report{
			ReportType:     watcher.ReportTypeADD,
			PipelineReport: report,
		}
	}
	return watcher.Report{
		ReportType: watcher.ReportTypeDELETE,
		PipelineReport: &watcher.PipelineReport{
			PgId:    pgID,
			NodeIds: nil,
			State:   0,
		},
	}
}

// makeAlayaRaftInPipeline Make a new raft node for a single pipeline (PG), it will:
//
// 1. Create a raft message chan if not exist
//
// 2. New an alaya raft node **but not run it**
//
// when leaderID is not zero, it while add to an existed raft group
func (a *Alaya) makeAlayaRaftInPipeline(p *pipeline.Pipeline, oldP *pipeline.Pipeline) *Raft {
	pgID := p.PgId
	c, ok := a.PGMessageChans.Load(pgID)
	if !ok {
		c = make(chan raftpb.Message)
		a.PGMessageChans.Store(pgID, c)
	}
	storage, err := a.MetaStorageRegister.NewStorage(pgID)
	if err != nil {
		logger.Fatalf("Alaya: %v, create meta storage for pg: %v err: %v", a.selfInfo.RaftId, pgID, err)
	}
	a.PGRaftNode.Store(pgID,
		NewAlayaRaft(a.ctx, a.selfInfo.RaftId, p, oldP, a.config, a.watcher, storage,
			c.(chan raftpb.Message), a.raftNodeStopChan))
	logger.Infof("Node: %v successful add raft node in alaya, PG: %v", a.selfInfo.RaftId, pgID)
	metrics.GetOrRegisterCounter(watcher.MetricsAlayaPipelineCount, nil).Inc(1)
	return a.getRaftNode(pgID)
}
