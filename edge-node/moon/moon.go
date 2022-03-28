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
	"google.golang.org/protobuf/testing/protocmp"
	"strconv"
	"sync"
	"time"
)

type Moon struct {
	// Moon Rpc
	UnimplementedMoonServer

	id            uint64 //raft节点的id
	SelfInfo      *infos.NodeInfo
	ctx           context.Context //context
	cancel        context.CancelFunc
	raftStorage   *raft.MemoryStorage //raft需要的内存结构
	stableStorage Storage
	cfg           *raft.Config //raft需要的配置
	raft          raft.Node
	ticker        <-chan time.Time //定时器，提供周期时钟源和超时触发能力
	infoMap       map[uint64]*infos.NodeInfo
	leaderID      uint64 // 注册时的 leader 信息

	infoStorageRegister   *infos.StorageRegister
	raftChan              chan raftpb.Message
	appliedRequestChan    chan *ProposeInfoRequest
	appliedConfChangeChan chan raftpb.ConfChange

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
	err = m.raft.Propose(ctx, data)

	// wait propose apply
	for {
		applied := <-m.appliedRequestChan
		if cmp.Equal(applied.BaseInfo, request.BaseInfo, protocmp.Transform()) {
			break
		} else {
			m.appliedRequestChan <- applied
			time.Sleep(100 * time.Millisecond)
		}
	}

	return &ProposeInfoReply{
		Result: &common.Result{
			Status: 0,
		},
		LeaderInfo: nil,
	}, err
}

func (m *Moon) GetInfo(_ context.Context, request *GetInfoRequest) (*GetInfoReply, error) {
	info, err := m.infoStorageRegister.Get(request.InfoType, request.InfoId)
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

func (m *Moon) ProposeConfChangeAddNode(ctx context.Context, nodeInfo *infos.NodeInfo) error {
	data, _ := nodeInfo.Marshal()
	err := m.raft.ProposeConfChange(ctx, raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  nodeInfo.RaftId,
		Context: data,
	})
	if err != nil {
		return err
	}
	// TODO: add time out
	<-m.appliedConfChangeChan
	return nil
}

func (m *Moon) SendRaftMessage(_ context.Context, message *raftpb.Message) (*raftpb.Message, error) {
	m.raftChan <- *message
	return &raftpb.Message{}, nil
}

func NewMoon(ctx context.Context, selfInfo *infos.NodeInfo, config *Config, rpcServer *messenger.RpcServer,
	register *infos.StorageRegister) *Moon {
	ctx, cancel := context.WithCancel(ctx)
	storage := raft.NewMemoryStorage()
	m := &Moon{
		id:                    0, // set raft id after register
		SelfInfo:              selfInfo,
		ctx:                   ctx,
		cancel:                cancel,
		raftStorage:           storage,
		stableStorage:         NewStorage(config.RaftStoragePath),
		cfg:                   nil, // set raft cfg after register
		ticker:                time.NewTicker(time.Millisecond * 200).C,
		mutex:                 sync.RWMutex{},
		infoMap:               make(map[uint64]*infos.NodeInfo),
		config:                config,
		status:                StatusInit,
		infoStorageRegister:   register,
		raftChan:              make(chan raftpb.Message),
		appliedRequestChan:    make(chan *ProposeInfoRequest, 100),
		appliedConfChangeChan: make(chan raftpb.ConfChange, 100),
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
			logger.Warningf("could not send raft message: %v", err)
		}

		err = conn.Close()
		if err != nil {
			logger.Errorf("close grpc conn err: %v", err)
		}
	}
}

func (m *Moon) IsLeader() bool {
	return m.raft.Status().Lead == m.id
}

func (m *Moon) process(entry raftpb.Entry) {
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		var msg ProposeInfoRequest
		err := msg.Unmarshal(entry.Data)
		if err != nil {
			logger.Errorf("unmarshal entry data fail: %v", err)
		}
		info := msg.BaseInfo
		switch msg.Operate {
		case ProposeInfoRequest_ADD:
			err = m.infoStorageRegister.Update(info)
		case ProposeInfoRequest_UPDATE:
			err = m.infoStorageRegister.Update(info)
		case ProposeInfoRequest_DELETE:
			err = m.infoStorageRegister.Delete(info.GetInfoType(), msg.Id)
		}
		if err != nil {
			logger.Errorf("Moon process moon message err: %v", err.Error())
		}
		// let request know it is already applied
		m.appliedRequestChan <- &msg
	}
}

func (m *Moon) Init(leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.status = StatusRegistering
	if leaderInfo != nil {
		m.infoMap[leaderInfo.RaftId] = leaderInfo
	}
	for _, nodeInfo := range peersInfo {
		m.infoMap[nodeInfo.RaftId] = nodeInfo
	}

	m.id = m.SelfInfo.RaftId
	cfg := raft.Config{
		ID:              m.SelfInfo.RaftId,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         m.raftStorage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}
	m.cfg = &cfg

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

	m.raft = raft.StartNode(m.cfg, peers)
	raft.SetLogger(logger.Logger)
}

func (m *Moon) Run() {
	if m.raft == nil {
		m.Init(nil, m.config.ClusterInfo.NodesInfo)
	}
	go m.reportSelfInfo()

	for {
		select {
		case <-m.ctx.Done():
			m.cleanup()
			return
		case <-m.ticker:
			m.raft.Tick()
		case rd := <-m.raft.Ready():
			// 将HardState，entries写入持久化存储中
			err := m.stableStorage.Save(rd.HardState, rd.Entries)
			if err != nil {
				logger.Errorf("save hardState of raft log entries fail")
				return
			}
			if !raft.IsEmptySnap(rd.Snapshot) {
				// 如果快照数据不为空，也需要保存快照数据到持久化存储中
				err = m.stableStorage.SaveSnap(rd.Snapshot)
				if err != nil {
					return
				}
				err = m.raftStorage.ApplySnapshot(rd.Snapshot)
				if err != nil {
					return
				}
			}
			_ = m.raftStorage.Append(rd.Entries)
			go m.sendByRpc(rd.Messages)
			for _, entry := range rd.CommittedEntries {
				m.process(entry)
				if entry.Type == raftpb.EntryConfChange {
					var cc raftpb.ConfChange
					_ = cc.Unmarshal(entry.Data)
					m.raft.ApplyConfChange(cc)
					if cc.Context != nil {
						m.appliedConfChangeChan <- cc
					}
				}
			}
			m.raft.Advance()
		case message := <-m.raftChan:
			_ = m.raft.Step(m.ctx, message)
		}
	}
}

func (m *Moon) Stop() {
	m.cancel()
}

func (m *Moon) cleanup() {
	logger.Warningf("moon %d stopped, start clean up", m.SelfInfo.RaftId)
	m.raft.Stop()
	m.stableStorage.Close()
}

func (m *Moon) reportSelfInfo() {
	for m.raft.Status().Lead == 0 {
		time.Sleep(1 * time.Second)
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
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.raft == nil {
		return 0
	}
	return m.raft.Status().Lead
}
