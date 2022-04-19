package moon

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"errors"
	"github.com/google/go-cmp/cmp"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"google.golang.org/protobuf/testing/protocmp"
	"strconv"
	"sync"
	"time"
)

type InfoController interface {
	MoonServer

	GetInfoDirect(infoType infos.InfoType, id string) (infos.Information, error)
	ProposeConfChangeAddNode(ctx context.Context, nodeInfo *infos.NodeInfo) error
	IsLeader() bool
	Set(selfInfo, leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo)
	GetLeaderID() uint64
	Stop()
	Run()
}

type Moon struct {
	// Moon Rpc
	UnimplementedMoonServer

	// Channels communication between Moon and Raft module
	proposeC       chan<- string            // channel for proposing updates
	confChangeC    chan<- raftpb.ConfChange // proposed cluster config changes
	communicationC <-chan []raftpb.Message
	commitC        <-chan *commit
	errorC         <-chan error
	nodeReady      bool

	id       uint64 //raft节点的id
	raft     *raftNode
	SelfInfo *infos.NodeInfo
	ctx      context.Context //context
	cancel   context.CancelFunc

	infoMap     map[uint64]*infos.NodeInfo
	infoStorage *infos.Storage
	leaderID    uint64 // 注册时的 leader 信息

	infoStorageRegister   *infos.StorageRegister
	appliedRequestChan    chan *ProposeInfoRequest
	appliedConfChangeChan chan raftpb.ConfChange
	appliedConfErrorChan  chan error

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

func (m *Moon) ProposeInfo(ctx context.Context, request *ProposeInfoRequest) (*ProposeInfoReply, error) {
	if request.Id == "" {
		return nil, errors.New("info key is empty")
	}

	data, err := request.Marshal()
	if err != nil {
		logger.Errorf("receive unmarshalled propose info request: %v", request.Id)
		return nil, err
	}

	m.proposeC <- string(data)

	// wait propose apply
	for {
		applied := <-m.appliedRequestChan
		if cmp.Equal(applied.BaseInfo, request.BaseInfo, protocmp.Transform()) {
			break
		} else {
			m.appliedRequestChan <- applied
			time.Sleep(10 * time.Millisecond)
		}
	}

	return &ProposeInfoReply{
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

func (m *Moon) readCommits(commitC <-chan *commit, errorC <-chan error) {
	for commit := range commitC {
		if commit == nil {
			snapshot, err := m.loadSnapshot()
			if err != nil {
				logger.Errorf("failed to load snapshot: %v", err)
			}
			if snapshot != nil {
				logger.Infof("loading snapshot at term %d and index %d", snapshot.Metadata.Term, snapshot.Metadata.Index)
				if err := m.infoStorageRegister.RecoverFromSnapshot(snapshot.Data); err != nil {
					logger.Errorf("failed to recover from snapshot: %v", err)
				}
			}
			continue
		}

		for _, rawData := range commit.data {
			var msg ProposeInfoRequest
			data := []byte(rawData)
			err := msg.Unmarshal(data)
			if err != nil {
				logger.Errorf("failed to unmarshal data: %v", err)
			}
			info := msg.BaseInfo
			switch msg.Operate {
			case ProposeInfoRequest_ADD:
				logger.Tracef("%d add info %v", m.id, info.GetID())
				err = m.infoStorageRegister.Update(info)

			case ProposeInfoRequest_UPDATE:
				err = m.infoStorageRegister.Update(info)
			case ProposeInfoRequest_DELETE:
				err = m.infoStorageRegister.Delete(info.GetInfoType(), msg.Id)
			}
			if err != nil {
				logger.Errorf("Moon process moon message err: %v", err.Error())
			}
			m.appliedRequestChan <- &msg
		}
		close(commit.applyDoneC) // TODO:
	}
	if err, ok := <-errorC; ok {
		logger.Fatalf("commit stream error: %v", err)
	}
}

func (m *Moon) GetInfo(_ context.Context, request *GetInfoRequest) (*GetInfoReply, error) {
	info, err := m.GetInfoDirect(request.InfoType, request.InfoId)
	if err != nil {
		logger.Warningf("get info from storage register fail: %v", err)
		return &GetInfoReply{
			Result: &common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			},
			BaseInfo: nil,
		}, err
	}
	return &GetInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		BaseInfo: info.BaseInfo(),
	}, nil
}

func (m *Moon) GetInfoDirect(infoType infos.InfoType, id string) (infos.Information, error) {
	info, err := m.infoStorageRegister.Get(infoType, id)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (m *Moon) ProposeConfChangeAddNode(ctx context.Context, nodeInfo *infos.NodeInfo) error {
	data, _ := nodeInfo.Marshal()

	m.confChangeC <- raftpb.ConfChange{
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
		case <-m.appliedConfChangeChan:
			// TODO:
			return nil
		}
	}
	return nil
}

func (m *Moon) SendRaftMessage(_ context.Context, message *raftpb.Message) (*raftpb.Message, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.raft == nil {
		return nil, errors.New("moon" + strconv.FormatUint(m.id, 10) + ": raft is not ready")
	}

	m.raft.raftChan <- *message

	return &raftpb.Message{}, nil
}

func NewMoon(ctx context.Context, selfInfo *infos.NodeInfo, config *Config, rpcServer *messenger.RpcServer,
	register *infos.StorageRegister) *Moon {

	proposeC := make(chan string) // TODO: close
	confChangeC := make(chan raftpb.ConfChange)
	communicationC := make(chan []raftpb.Message)

	ctx, cancel := context.WithCancel(ctx)

	m := &Moon{

		proposeC:       proposeC,
		confChangeC:    confChangeC,
		communicationC: communicationC,

		id:       0, // set raft id after register
		SelfInfo: selfInfo,
		ctx:      ctx,
		cancel:   cancel,

		mutex:                 sync.RWMutex{},
		infoMap:               make(map[uint64]*infos.NodeInfo),
		config:                config,
		status:                StatusInit,
		infoStorageRegister:   register,
		appliedRequestChan:    make(chan *ProposeInfoRequest, 100),
		appliedConfChangeChan: make(chan raftpb.ConfChange, 100),
		appliedConfErrorChan:  make(chan error),
	}
	leaderInfo := config.ClusterInfo.LeaderInfo
	if leaderInfo != nil {
		m.leaderID = leaderInfo.RaftId
	}

	RegisterMoonServer(rpcServer, m)

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
		logger.Tracef("%d send to %v, type %v", m.id, message, message.Type)

		// get node info
		var nodeInfo *infos.NodeInfo
		var ok bool
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
		c := NewMoonClient(conn)
		_, err = c.SendRaftMessage(m.ctx, &message)
		if err != nil {
			m.appliedConfErrorChan <- err
			logger.Warningf("could not send raft message: %v", err)
		}
	}
}

func (m *Moon) IsLeader() bool {
	return m.raft.node.Status().Lead == m.id
}

func (m *Moon) Set(selfInfo, leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.status = StatusRegistering
	m.SelfInfo = selfInfo
	if leaderInfo != nil {
		m.infoMap[leaderInfo.RaftId] = leaderInfo
	}
	for _, nodeInfo := range peersInfo {
		m.infoMap[nodeInfo.RaftId] = nodeInfo
	}

	m.id = m.SelfInfo.RaftId

	var peers []raft.Peer

	m.infoMap[m.SelfInfo.RaftId] = m.SelfInfo

	logger.Tracef("leaderInfo: %v", leaderInfo)

	join := false

	if leaderInfo != nil { // 集群中现在已经有成员，peers 只需要填写 leader
		peers = []raft.Peer{
			{ID: leaderInfo.RaftId},
		}
		if peersInfo != nil {
			join = true
		}
	} else {
		for _, nodeInfo := range m.infoMap {
			peers = append(peers, raft.Peer{ID: nodeInfo.RaftId})
		}
	}

	//if peersInfo != nil && leaderInfo != nil {
	//	join = true
	//}
	//
	//for _, nodeInfo := range m.infoMap {
	//	peers = append(peers, raft.Peer{ID: nodeInfo.RaftId})
	//}

	proposeC, confChangeC, communicationC, commitC, errorC, nodeReadyC, snapshotterReady, raftNode := newRaftNode(int(m.id), peers, join, m.config.RaftStoragePath, m.infoStorageRegister.GetSnapshot)
	m.proposeC = proposeC
	m.confChangeC = confChangeC
	m.communicationC = communicationC
	m.raft = raftNode
	m.commitC = commitC
	m.errorC = errorC
	m.nodeReady = <-nodeReadyC
	m.snapshotter = <-snapshotterReady
	return
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
	go m.readCommits(m.commitC, m.errorC)

	for {
		select {
		case <-m.ctx.Done():
			m.cleanup()
			return
		case msgs := <-m.communicationC:
			go m.sendByRpc(msgs)
		}
	}
}

func (m *Moon) Stop() {
	m.cancel()
}

func (m *Moon) cleanup() {
	logger.Warningf("moon %d stopped, start clean up", m.SelfInfo.RaftId)
	m.raft.stopc <- struct{}{}
}

func (m *Moon) reportSelfInfo() {

	for {
		if m.nodeReady == true {
			break
		}
	}

	for m.raft.node.Status().Lead == 0 {
		time.Sleep(100 * time.Millisecond)
	}

	logger.Infof("%v join group success, start report self info", m.id)
	message := &ProposeInfoRequest{
		Head: &common.Head{
			Timestamp: timestamp.Now(),
			Term:      0,
		},
		Id:       strconv.FormatUint(m.SelfInfo.RaftId, 10),
		Operate:  ProposeInfoRequest_UPDATE,
		BaseInfo: &infos.BaseInfo{Info: &infos.BaseInfo_NodeInfo{NodeInfo: m.SelfInfo}},
	}
	_, err := m.ProposeInfo(m.ctx, message)
	if err != nil {
		logger.Errorf("report self info err: %v", err.Error())
	}
	m.status = StatusOK
}

func (m *Moon) GetLeaderID() uint64 {
	for {
		m.mutex.Lock()
		nodeReady := m.nodeReady
		m.mutex.Unlock()
		if nodeReady == true {
			break
		}
	}

	if m.raft.node == nil {
		return 0
	}
	return m.raft.node.Status().Lead
}
