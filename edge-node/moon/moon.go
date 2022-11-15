package moon

import (
	"context"
	"ecos/edge-node/infos"
	eraft "ecos/edge-node/raft-node"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/moon"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"errors"
	"go.etcd.io/etcd/pkg/v3/idutil"
	"go.etcd.io/etcd/pkg/v3/wait"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"strconv"
	"sync"
	"time"
)

type InfoController interface {
	moon.MoonServer

	GetInfoDirect(infoType infos.InfoType, id string) (infos.Information, error)
	ProposeConfChangeAddNode(ctx context.Context, nodeInfo *infos.NodeInfo) error
	NodeInfoChanged(nodeInfo *infos.NodeInfo)
	IsLeader() bool
	Set(selfInfo, leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo)
	GetLeaderID() uint64
	Stop()
	Run()
}

type Moon struct {
	// Moon Rpc
	moon.UnimplementedMoonServer

	id        uint64 //raft节点的id
	raft      *eraft.RaftNode
	SelfInfo  *infos.NodeInfo
	ctx       context.Context //context
	cancel    context.CancelFunc
	nodeReady bool // Channels communication between Moon and Raft module

	infoMap  map[uint64]*infos.NodeInfo
	leaderID uint64 // 注册时的 leader 信息

	infoStorageRegister *infos.StorageRegister
	//appliedRequestChan   chan *ProposeInfoRequest
	w                    wait.Wait
	reqIDGen             *idutil.Generator
	appliedConfErrorChan chan error

	snapshotter *snap.Snapshotter

	mutex  sync.RWMutex
	config *Config
	status Status
}

type ActionType int

type Message2 struct {
	Action    ActionType
	NodeInfo  infos.NodeInfo
	Term      uint64
	TimeStamp *timestamp.Timestamp
}

type Status int

const (
	StatusInit Status = iota
	StatusRegistering
	StatusOK
)

func (m *Moon) ProposeInfo(ctx context.Context, request *moon.ProposeInfoRequest) (*moon.ProposeInfoReply, error) {
	if request.Id == "" {
		return nil, errors.New("info key is empty")
	}

	opID := m.reqIDGen.Next()
	request.OperateId = opID
	data, err := request.Marshal()
	if err != nil {
		logger.Errorf("receive unmarshalled propose info request: %v", request.Id)
		return nil, err
	}

	// 注册
	ch := m.w.Register(opID)
	m.raft.ProposeC <- data

	// wait propose apply
	select {
	case <-ch:
		logger.Infof("propose info request: %v SUCCESS", request.Id)
	}

	return &moon.ProposeInfoReply{
		Result: &common.Result{
			Status: 0,
		},
		LeaderInfo: nil,
	}, err
}

func (m *Moon) loadSnapshot() (*raftpb.Snapshot, error) {
	snapshot, err := m.snapshotter.Load()
	if err == snap.ErrNoSnapshot {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (m *Moon) readCommits(commitC <-chan *eraft.Commit, errorC <-chan error) {
	for commit := range commitC {
		if commit == nil {
			snapshot, err := m.loadSnapshot()
			if err != nil {
				logger.Errorf("failed to load snapshot: %v", err)
			}
			if snapshot != nil {
				logger.Infof("%d loading snapshot at term %d and index %d", m.id, snapshot.Metadata.Term, snapshot.Metadata.Index)
				if err := m.infoStorageRegister.RecoverFromSnapshot(snapshot.Data); err != nil {
					logger.Errorf("[%v] failed to recover from snapshot: %v", m.id, err)
				} else {
					logger.Infof("[%v] recover from snapshot success", m.id)
				}
			}
			continue
		}

		for _, rawData := range commit.Data {
			var msg moon.ProposeInfoRequest
			data := []byte(rawData)
			err := msg.Unmarshal(data)
			if err != nil {
				logger.Errorf("failed to unmarshal data: %v", err)
			}
			info := msg.BaseInfo
			switch msg.Operate {
			case moon.ProposeInfoRequest_ADD:
				logger.Tracef("%d add info %v", m.id, info.GetID())
				err = m.infoStorageRegister.Update(info)

			case moon.ProposeInfoRequest_UPDATE:
				err = m.infoStorageRegister.Update(info)
			case moon.ProposeInfoRequest_DELETE:
				err = m.infoStorageRegister.Delete(info.GetInfoType(), msg.Id)
			}
			if err != nil {
				logger.Errorf("Moon process moon message err: %v", err.Error())
			}
			if m.w.IsRegistered(msg.OperateId) {
				m.w.Trigger(msg.OperateId, struct{}{})
			}
		}
		close(commit.ApplyDoneC)
	}
	if err, ok := <-errorC; ok {
		logger.Fatalf("commit stream error: %v", err)
	}
}

func (m *Moon) GetInfo(_ context.Context, request *moon.GetInfoRequest) (*moon.GetInfoReply, error) {
	info, err := m.GetInfoDirect(request.InfoType, request.InfoId)
	if err != nil {
		logger.Warningf("get info from storage register fail: %v", err)
		return &moon.GetInfoReply{
			Result: &common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			},
			BaseInfo: nil,
		}, err
	}
	return &moon.GetInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		BaseInfo: info.BaseInfo(),
	}, nil
}

func (m *Moon) ListInfo(ctx context.Context, request *moon.ListInfoRequest) (*moon.ListInfoReply, error) {
	if m.isStopped() {
		return nil, errors.New("moon is stopped")
	}
	result, err := m.infoStorageRegister.List(request.InfoType, request.Prefix)
	if err != nil {
		logger.Warningf("list info from storage register fail: %v", err)
		return nil, err
	}
	baseInfos := make([]*infos.BaseInfo, 0, len(result))
	for _, info := range result {
		baseInfos = append(baseInfos, info.BaseInfo())
	}
	return &moon.ListInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		BaseInfos: baseInfos,
	}, nil
}

func (m *Moon) GetInfoDirect(infoType infos.InfoType, id string) (infos.Information, error) {
	if m.isStopped() {
		return nil, errors.New("moon is stopped")
	}
	info, err := m.infoStorageRegister.Get(infoType, id)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (m *Moon) ProposeConfChangeAddNode(ctx context.Context, nodeInfo *infos.NodeInfo) error {
	data, _ := nodeInfo.Marshal()
	logger.Infof("ProposeConfChangeAddNode: %v", nodeInfo)

	m.raft.ConfChangeC <- raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  nodeInfo.RaftId,
		Context: data,
	}

	// TODO: add time out
	for {
		select {
		case <-m.appliedConfErrorChan:
			// TODO:
			return nil
		case <-m.raft.ApplyConfChangeC:
			// TODO:
			return nil
		}
	}
	return nil
}

func (m *Moon) NodeInfoChanged(nodeInfo *infos.NodeInfo) {
	m.infoMap[nodeInfo.RaftId] = nodeInfo
}

func (m *Moon) SendRaftMessage(_ context.Context, message *raftpb.Message) (*raftpb.Message, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.raft == nil {
		return nil, errors.New("moon" + strconv.FormatUint(m.id, 10) + ": raft is not ready")
	}

	m.raft.RaftChan <- *message

	return &raftpb.Message{}, nil
}

func NewMoon(ctx context.Context, selfInfo *infos.NodeInfo, config *Config, rpcServer *messenger.RpcServer,
	register *infos.StorageRegister) *Moon {

	ctx, cancel := context.WithCancel(ctx)

	m := &Moon{
		id:                   0, // set raft id after register
		SelfInfo:             selfInfo,
		ctx:                  ctx,
		cancel:               cancel,
		mutex:                sync.RWMutex{},
		infoMap:              make(map[uint64]*infos.NodeInfo),
		config:               config,
		status:               StatusInit,
		infoStorageRegister:  register,
		w:                    wait.New(),
		reqIDGen:             idutil.NewGenerator(0, time.Now()),
		appliedConfErrorChan: make(chan error),
	}
	leaderInfo := config.ClusterInfo.LeaderInfo
	if leaderInfo != nil {
		m.leaderID = leaderInfo.RaftId
	}

	moon.RegisterMoonServer(rpcServer, m)

	if leaderInfo != nil {
		m.infoMap[leaderInfo.RaftId] = leaderInfo
	}
	for _, info := range config.ClusterInfo.NodesInfo {
		m.infoMap[info.RaftId] = info
	}

	return m
}

func (m *Moon) sendByRpc(messages []raftpb.Message) {
	for _, message := range messages {
		select {
		case <-m.ctx.Done():
			return
		default:
		}
		logger.Tracef("%d send to %v, type %v", m.id, message, message.Type)

		if message.Type == raftpb.MsgSnap {
			message.Snapshot.Metadata.ConfState = m.raft.ConfState
		}

		// get node info
		var nodeInfo *infos.NodeInfo
		var ok bool
		select {
		case <-m.ctx.Done():
			logger.Warningf("moon %d: context is done", m.id)
			return
		default:
		}
		if nodeInfo, ok = m.infoMap[message.To]; !ok { // infoMap always have latest
			storage := m.infoStorageRegister.GetStorage(infos.InfoType_NODE_INFO)
			info, err := storage.Get(strconv.FormatUint(message.To, 10))
			if err != nil {
				logger.Warningf("Get Node Info fail: %v", err)
				return
			}
			nodeInfo = info.BaseInfo().GetNodeInfo()
		}

		conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
		if err != nil {
			logger.Errorf("failed to connect: %v", err)
			return
		}
		c := moon.NewMoonClient(conn)
		_, err = c.SendRaftMessage(m.ctx, &message)
		if err != nil {
			m.appliedConfErrorChan <- err
			logger.Warningf("could not send raft message: %v", err)
		}
	}
}

func (m *Moon) IsLeader() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.raft == nil || m.raft.Node == nil {
		return false
	}
	return m.raft.Node.Status().Lead == m.id
}

func (m *Moon) Set(selfInfo, leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.status = StatusRegistering
	m.SelfInfo = selfInfo
	for _, nodeInfo := range peersInfo {
		m.infoMap[nodeInfo.RaftId] = nodeInfo
	}
	if leaderInfo != nil { // leader info is the highest priority
		m.infoMap[leaderInfo.RaftId] = leaderInfo
	}

	m.id = m.SelfInfo.RaftId

	var peers []raft.Peer

	m.infoMap[m.SelfInfo.RaftId] = m.SelfInfo

	logger.Tracef("leaderInfo: %v", leaderInfo)

	if leaderInfo != nil { // 集群中现在已经有成员，peers 只需要填写 leader
		peers = []raft.Peer{
			{ID: leaderInfo.RaftId},
		}
	} else {
		for _, nodeInfo := range m.infoMap {
			peers = append(peers, raft.Peer{ID: nodeInfo.RaftId})
		}
	}
	readyC := make(chan bool)
	snapshotterReady, raftNode := eraft.NewRaftNode(int(m.id), m.ctx, peers, m.config.RaftStoragePath, readyC, m.infoStorageRegister.GetSnapshot)
	m.raft = raftNode
	m.nodeReady = <-readyC
	m.snapshotter = <-snapshotterReady
}

func (m *Moon) Run() {
	m.mutex.Lock()
	raft := m.raft
	m.mutex.Unlock()
	if raft == nil {
		m.Set(m.SelfInfo, nil, m.config.ClusterInfo.NodesInfo)
	}
	go m.reportSelfInfo()

	// read commits from raft into storage until error
	go m.readCommits(m.raft.CommitC, m.raft.ErrorC)

	for {
		select {
		case <-m.ctx.Done():
			m.cleanup()
			return
		case msgs := <-m.raft.CommunicationC:
			go m.sendByRpc(msgs)
		case <-m.raft.ApplyConfChangeC:
			// DO NOTHING
		}
	}
}

func (m *Moon) Stop() {
	m.cancel()
}

func (m *Moon) cleanup() {
	logger.Warningf("moon %d stopped, start clean up", m.SelfInfo.RaftId)
	m.infoStorageRegister.Close()
	logger.Warningf("moon %d clean up done", m.SelfInfo.RaftId)
}

func (m *Moon) isStopped() bool {
	select {
	case <-m.ctx.Done():
		return true
	default:
		return false
	}
}

func (m *Moon) reportSelfInfo() {
	logger.Infof("%v join group success, start report self info", m.id)
	message := &moon.ProposeInfoRequest{
		Head: &common.Head{
			Timestamp: timestamp.Now(),
			Term:      0,
		},
		Id:        strconv.FormatUint(m.SelfInfo.RaftId, 10),
		Operate:   moon.ProposeInfoRequest_UPDATE,
		OperateId: 0,
		BaseInfo:  &infos.BaseInfo{Info: &infos.BaseInfo_NodeInfo{NodeInfo: m.SelfInfo}},
	}
	_, err := m.ProposeInfo(m.ctx, message)
	if err != nil {
		logger.Errorf("report self info err: %v", err.Error())
	}
	m.status = StatusOK
}

func (m *Moon) GetLeaderID() uint64 {
	for {
		m.mutex.RLock()
		nodeReady := m.nodeReady
		m.mutex.RUnlock()
		if nodeReady {
			break
		}
	}
	if m.raft.Node == nil {
		return 0
	}
	return m.raft.Node.Status().Lead
}
