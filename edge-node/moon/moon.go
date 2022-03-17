package moon

import (
	"context"
	"ecos/cloud/sun"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"encoding/json"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
	"time"
)

type Moon struct {
	id            uint64 //raft节点的id
	selfInfo      *node.NodeInfo
	ctx           context.Context //context
	cancel        context.CancelFunc
	InfoStorage   node.InfoStorage
	raftStorage   *raft.MemoryStorage //raft需要的内存结构
	stableStorage Storage
	cfg           *raft.Config //raft需要的配置
	raft          raft.Node
	ticker        <-chan time.Time //定时器，提供周期时钟源和超时触发能力
	infoMap       map[uint64]*node.NodeInfo
	leaderID      uint64 // 注册时的 leader 信息

	// InfoStorageTimer trigger infoStorage to commit
	InfoStorageTimer *time.Timer

	// Moon Rpc
	UnimplementedMoonServer

	sunAddr  string
	raftChan chan raftpb.Message

	mutex  sync.Mutex
	config *Config
}

type ActionType int

type Message struct {
	Action    ActionType
	NodeInfo  node.NodeInfo
	Term      uint64
	TimeStamp *timestamp.Timestamp
}

const (
	UpdateNodeInfo ActionType = iota
	DeleteNodeInfo
	StorageCommit
	StorageApply
)

func (m *Moon) AddNodeToGroup(_ context.Context, info *node.NodeInfo) (*AddNodeReply, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	reply := AddNodeReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		LeaderInfo: nil,
	}
	msg := Message{
		Action:    UpdateNodeInfo,
		NodeInfo:  *info,
		Term:      m.InfoStorage.GetTermNow(),
		TimeStamp: timestamp.Now(),
	}
	js, _ := json.Marshal(&msg)
	if m.raft == nil {
		reply.Result.Status = common.Result_FAIL
		reply.Result.Message = errno.MoonRaftNotReady.Error()
		return &reply, errno.MoonRaftNotReady
	}
	err := m.raft.Propose(m.ctx, js)
	if err != nil {
		reply.Result.Status = common.Result_FAIL
		reply.Result.Message = "propose node info fail"
		return &reply, err
	}
	logger.Infof("send propose node info success, start wait")
	// TODO (zhang): check it by channel
	isReady := false
	for i := 0; i < 10; i++ {
		nodeInfo, err := m.InfoStorage.GetNodeInfo(node.ID(info.RaftId))
		if err != nil || info.Uuid != nodeInfo.Uuid {
			time.Sleep(1 * time.Second)
		} else {
			isReady = true
			break
		}
	}
	if !isReady {
		reply.Result.Status = common.Result_FAIL
		reply.Result.Message = "propose conf change time out"
		logger.Warningf("propose conf change time out")
		return &reply, err
	}
	logger.Infof("Add new node info %v success", info.RaftId)

	err = m.raft.ProposeConfChange(m.ctx, raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  info.RaftId,
		Context: nil,
	})

	if err != nil {
		reply.Result.Status = common.Result_FAIL
		reply.Result.Message = "propose conf change fail"
		return &reply, err
	}

	return &reply, nil
}

func (m *Moon) SendRaftMessage(_ context.Context, message *raftpb.Message) (*raftpb.Message, error) {
	m.raftChan <- *message
	return &raftpb.Message{}, nil
}

func (m *Moon) RequestJoinGroup(leaderInfo *node.NodeInfo) error {
	tryTime := 3
	var fail error
	var conn *grpc.ClientConn
	conn, err := messenger.GetRpcConn(leaderInfo.IpAddr, leaderInfo.RpcPort)
	if err != nil {
		logger.Warningf("Request Join group err: %v", err.Error())
		fail = err
		return fail
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)
	for i := 0; i < tryTime; i++ {
		if i > 0 {
			time.Sleep(time.Second)
		}
		var err error
		client := NewMoonClient(conn)
		result, err := client.AddNodeToGroup(context.Background(), m.selfInfo)
		if err != nil {
			logger.Warningf("Request Join group err: %v", err.Error())
			fail = err
			continue
		}
		if result.Result.Status != common.Result_OK {
			// 检查是否该节点不是 leader, 需要重定向到新 leader
			// Check the new leader
			if result.LeaderInfo != nil {
				time.Sleep(2 * time.Second)
				err = m.RequestJoinGroup(result.LeaderInfo)
				if err != nil {
					return err
				}
			}
			return err
		}
		break
	}
	if fail != nil {
		return fail
	}
	return nil
}

func (m *Moon) Register(sunAddr string) (leaderInfo *node.NodeInfo, err error) {
	if sunAddr == "" {
		return nil, errno.ConnectSunFail
	}
	conn, err := grpc.Dial(sunAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	c := sun.NewSunClient(conn)
	result, err := c.MoonRegister(context.Background(), m.selfInfo)
	if err != nil {
		return nil, err
	}
	m.selfInfo.RaftId = result.RaftId

	if result.HasLeader {
		m.infoMap[result.GroupInfo.LeaderInfo.RaftId] = result.GroupInfo.LeaderInfo
		err = m.RequestJoinGroup(result.GroupInfo.LeaderInfo)
		if err != nil {
			return result.GroupInfo.LeaderInfo, err
		}
	}

	for _, nodeInfo := range result.GroupInfo.NodesInfo {
		m.infoMap[nodeInfo.RaftId] = nodeInfo
	}

	return result.GroupInfo.LeaderInfo, nil
}

func NewMoon(selfInfo *node.NodeInfo, config *Config, rpcServer *messenger.RpcServer,
	infoStorage node.InfoStorage, stableStorage Storage) *Moon {
	ctx, cancel := context.WithCancel(context.Background())
	storage := raft.NewMemoryStorage()
	raftChan := make(chan raftpb.Message)
	sunAddr := config.SunAddr
	m := &Moon{
		id:            0, // set raft id after register
		selfInfo:      selfInfo,
		ctx:           ctx,
		cancel:        cancel,
		InfoStorage:   infoStorage,
		raftStorage:   storage,
		stableStorage: stableStorage,
		cfg:           nil, // set raft cfg after register
		ticker:        time.NewTicker(time.Millisecond * 100).C,
		raftChan:      raftChan,
		sunAddr:       sunAddr,
		mutex:         sync.Mutex{},
		infoMap:       make(map[uint64]*node.NodeInfo),
		config:        config,
	}
	leaderInfo := config.GroupInfo.LeaderInfo
	if leaderInfo != nil {
		m.leaderID = leaderInfo.RaftId
	}

	RegisterMoonServer(rpcServer, m)

	if leaderInfo != nil {
		m.infoMap[leaderInfo.RaftId] = leaderInfo
	}
	for _, info := range config.GroupInfo.NodesInfo {
		m.infoMap[info.RaftId] = info
	}
	return m
}

func (m *Moon) sendByRpc(messages []raftpb.Message) {
	var err error
	for _, message := range messages {
		logger.Tracef("%d send to %v, type %v", m.id, message, message.Type)

		// get node info
		nodeId := node.ID(message.To)
		var nodeInfo *node.NodeInfo
		var ok bool
		if nodeInfo, ok = m.infoMap[message.To]; !ok { // infoMap always have latest
			nodeInfo, err = m.InfoStorage.GetNodeInfo(nodeId) // else get from infoStorage
			if err != nil {
				logger.Warningf("Get Node Info fail: %v", err)
				return
			}
		}

		conn, err := messenger.GetRpcConnByInfo(nodeInfo)
		if err != nil {
			logger.Errorf("faild to connect: %v", err)
			return
		}
		c := NewMoonClient(conn)
		_, err = c.SendRaftMessage(context.Background(), &message)
		if err != nil {
			logger.Errorf("could not send raft message: %v", err)
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

// waitAndCommitStorage start a timer to wait a NodeInfoCommitInterval,
// after that start to propose a StorageCommit log. (if this node is leader)
func (m *Moon) waitAndCommitStorage() {
	if !m.IsLeader() {
		return // only leader can propose storage apply
	}
	if m.InfoStorageTimer == nil || !m.InfoStorageTimer.Stop() {
		m.InfoStorageTimer = time.AfterFunc(m.config.NodeInfoCommitInterval, func() {
			message := Message{
				Action:    StorageCommit,
				NodeInfo:  node.NodeInfo{},
				Term:      uint64(time.Now().UnixNano()),
				TimeStamp: timestamp.Now(),
			}
			data, _ := json.Marshal(&message)
			err := m.raft.Propose(m.ctx, data)
			if err != nil {
				logger.Errorf("Moon: %v propose storage commit term: %v err: %v", m.id, message.Term, err)
			}
			logger.Infof("Moon: %v propose storage commit term: %v", m.id, message.Term)
		})
	} else {
		m.InfoStorageTimer.Reset(m.config.NodeInfoCommitInterval)
	}
}

// waitAndCommitStorage wait all follower finish InfoStorage commit,
// and start to propose a StorageApply log. (if this node is leader)
func (m *Moon) waitAndApplyStorage(commitMessage *Message) {
	if !m.IsLeader() {
		return // only leader can propose storage commit
	}
	// TODO: wait all follower ready
	message := Message{
		Action:    StorageApply,
		NodeInfo:  node.NodeInfo{},
		Term:      commitMessage.Term,
		TimeStamp: timestamp.Now(),
	}
	data, _ := json.Marshal(&message)
	err := m.raft.Propose(m.ctx, data)
	if err != nil {
		logger.Errorf("Moon: %v propose storage apply term: %v err: %v", m.id, message.Term, err)
	}
	logger.Infof("Moon: %v propose storage apply term: %v", m.id, message.Term)
}

func (m *Moon) process(entry raftpb.Entry) {
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		var msg Message
		err := json.Unmarshal(entry.Data, &msg)
		switch msg.Action {
		case UpdateNodeInfo:
			nodeInfo := msg.NodeInfo
			logger.Infof("Node %v: get Moon info %v", m.id, &nodeInfo)
			_ = m.InfoStorage.UpdateNodeInfo(&nodeInfo, msg.TimeStamp)
			m.waitAndCommitStorage()
			break
		case DeleteNodeInfo:
			nodeInfo := msg.NodeInfo
			logger.Infof("Node %v: get Moon info %v", m.id, &nodeInfo)
			_ = m.InfoStorage.DeleteNodeInfo(node.ID(nodeInfo.RaftId), msg.TimeStamp)
			m.waitAndCommitStorage()
			break
		case StorageCommit:
			m.InfoStorage.Commit(msg.Term)
			m.waitAndApplyStorage(&msg)
			break
		case StorageApply:
			m.InfoStorage.Apply()
			break
		}
		if err != nil {
			logger.Errorf("Moon process moon message err: %v", err.Error())
		}
	}
}

func (m *Moon) Init() error {
	var err error
	var leaderInfo *node.NodeInfo
	if m.leaderID != 0 {
		leaderInfo, _ = m.infoMap[m.leaderID]
		err = m.RequestJoinGroup(leaderInfo)
		if err != nil {
			logger.Errorf("Node %v: request join to group err, leader: %v", m.id, m.leaderID)
		}
	}
	if m.sunAddr != "" {
		leaderInfo, err = m.Register(m.sunAddr)
		if err != nil {
			logger.Warningf("Register to Sun err: %v", err)
		}
	}

	m.id = m.selfInfo.RaftId
	cfg := raft.Config{
		ID:              m.selfInfo.RaftId,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         m.raftStorage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}
	m.cfg = &cfg

	var peers []raft.Peer

	m.infoMap[m.selfInfo.RaftId] = m.selfInfo

	for _, nodeInfo := range m.infoMap {
		if nodeInfo.RaftId != m.id {
			// 非常奇怪，除了第一个节点之外，其他节点不能有集群的完整信息，否则后续 propose 无法被提交
			peers = append(peers, raft.Peer{
				ID:      nodeInfo.RaftId,
				Context: nil,
			})
		}
	}
	logger.Tracef("leaderInfo: %v", leaderInfo)

	if leaderInfo == nil || leaderInfo.RaftId == m.id {
		// 非常奇怪，还必须得保证在只有一个节点的时候，peers 得加入自身，否则选不出 leader
		peers = append(peers, raft.Peer{
			ID:      m.id,
			Context: nil,
		})
	}

	m.raft = raft.StartNode(m.cfg, peers)
	return nil
}

func (m *Moon) Run() {
	err := m.Init()
	if err != nil {
		logger.Fatalf("init moon err: %v", err)
		return
	}
	go m.reportSelfInfo()

	for {
		select {
		case <-m.ctx.Done():
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
				}
			}
			m.raft.Advance()
		case message := <-m.raftChan:
			//logger.Infof("%d got message from %v to %v, type %v", m.id, message.From, message.To, message.Type)
			_ = m.raft.Step(m.ctx, message)
		}
	}
}

func (m *Moon) Stop() {
	m.cancel()
	m.stableStorage.Close()
	m.InfoStorage.Close()
}

func (m *Moon) reportSelfInfo() {
	for m.raft.Status().Lead == 0 {
		time.Sleep(1 * time.Second)
	}

	logger.Infof("%v join group success, start report self info", m.id)
	message := Message{
		Action:    UpdateNodeInfo,
		NodeInfo:  *m.selfInfo,
		Term:      m.InfoStorage.GetTermNow(),
		TimeStamp: timestamp.Now(),
	}
	js, _ := json.Marshal(message)
	err := m.raft.Propose(m.ctx, js)
	if err != nil {
		logger.Errorf("report self info err: %v", err.Error())
	}
}
