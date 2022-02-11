package moon

import (
	"context"
	"ecos/cloud/sun"
	"ecos/edge-node/node"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/logger"
	"encoding/json"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/golang/glog"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"strconv"
	"time"
)

type Moon struct {
	id          uint64 //raft节点的id
	selfInfo    *node.NodeInfo
	ctx         context.Context //context
	cancel      context.CancelFunc
	InfoStorage node.InfoStorage
	raftStorage *raft.MemoryStorage //raft需要的内存结构
	storage     Storage
	cfg         *raft.Config //raft需要的配置
	raft        raft.Node
	ticker      <-chan time.Time //定时器，提供周期时钟源和超时触发能力

	// Moon Rpc
	UnimplementedMoonServer

	sunAddr  string
	raftChan chan raftpb.Message
}

func (m *Moon) AddNodeToGroup(_ context.Context, info *node.NodeInfo) (*AddNodeReply, error) {
	//if m.raft.Status().Lead != m.id {
	//	return &AddNodeReply{
	//		Result: &common.Result{
	//			Status:  common.Result_FAIL,
	//			Message: "I am not leader",
	//		},
	//		LeaderInfo: nil,
	//	}, nil
	//}

	reply := AddNodeReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		LeaderInfo: nil,
	}

	js, _ := json.Marshal(info)
	err := m.raft.Propose(m.ctx, js)
	if err != nil {
		reply.Result.Status = common.Result_FAIL
		reply.Result.Message = "propose node info fail"
		return &reply, err
	}
	logger.Infof("send propose node info success, start wait")
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
	conn, err := messenger.GetRpcConn(leaderInfo.IpAddr, leaderInfo.RpcPort)
	if err != nil {
		logger.Warningf("Request Join group err: %v", err.Error())
		return err
	}
	defer conn.Close()
	client := NewMoonClient(conn)
	result, err := client.AddNodeToGroup(context.Background(), m.selfInfo)
	if err != nil {
		logger.Warningf("Request Join group err: %v", err.Error())
	}
	if result.Result.Status == common.Result_OK {
		return err
	}

	// Check the new leader
	if result.LeaderInfo != nil {
		time.Sleep(2 * time.Second)
		m.RequestJoinGroup(result.LeaderInfo)
	}
	return nil

}

func (m *Moon) Register(sunAddr string) error {

	conn, err := grpc.Dial(sunAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	c := sun.NewSunClient(conn)
	result, err := c.MoonRegister(context.Background(), m.selfInfo)
	if err != nil {
		return err
	}
	m.selfInfo.RaftId = result.RaftId

	if result.HasLeader {
		m.InfoStorage.UpdateNodeInfo(result.GroupInfo.LeaderInfo)
		err := m.RequestJoinGroup(result.GroupInfo.LeaderInfo)
		if err != nil {
			return err
		}
	}

	for _, nodeInfo := range result.GroupInfo.NodesInfo {
		_ = m.InfoStorage.UpdateNodeInfo(nodeInfo)
	}

	return nil
}

func NewMoon(selfInfo *node.NodeInfo, sunAddr string,
	leaderInfo *node.NodeInfo, groupInfo []*node.NodeInfo, rpcServer *messenger.RpcServer) *Moon {
	ctx, cancel := context.WithCancel(context.Background())
	storage := raft.NewMemoryStorage()
	infoStorage := node.NewMemoryNodeInfoStorage()
	raftChan := make(chan raftpb.Message)
	storagePath := "../../ecos-data/db/"
	stableStorage := NewStorage(storagePath)
	m := &Moon{
		id:          0, // set raft id after register
		selfInfo:    selfInfo,
		ctx:         ctx,
		cancel:      cancel,
		InfoStorage: infoStorage,
		raftStorage: storage,
		storage:     stableStorage,
		cfg:         nil, // set raft cfg after register
		ticker:      time.Tick(time.Millisecond * 100),
		raftChan:    raftChan,
	}

	registerSuccess := false
	if sunAddr != "" {
		err := m.Register(sunAddr)
		if err != nil {
			logger.Errorf("Register to Sun err: %v", err)
			registerSuccess = false
		} else {
			registerSuccess = true
		}
	}

	m.id = selfInfo.RaftId
	cfg := raft.Config{
		ID:              selfInfo.RaftId,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}
	m.cfg = &cfg

	RegisterMoonServer(rpcServer, m)

	var peers []raft.Peer

	allNodeInfo := m.InfoStorage.ListAllNodeInfo()
	if len(allNodeInfo) > 0 {
		logger.Infof("Node: %v Get GroupInfo from leader", m.id)
	} else {
		logger.Infof("Node: %v Get groupInfo form param", m.id)
		for _, nodeInfo := range groupInfo {
			info, err := infoStorage.GetNodeInfo(node.ID(nodeInfo.RaftId))
			if err != nil || info == nil { // node info not in InfoStorage
				_ = infoStorage.UpdateNodeInfo(nodeInfo)
			}
		}
	}

	m.InfoStorage.UpdateNodeInfo(selfInfo)
	allNodeInfo = m.InfoStorage.ListAllNodeInfo()

	for _, nodeInfo := range allNodeInfo {
		if nodeInfo.RaftId != m.id {
			// 非常奇怪，除了第一个节点之外，其他节点不能有集群的完整信息，否则后续 propose 无法被提交
			peers = append(peers, raft.Peer{
				ID:      nodeInfo.RaftId,
				Context: nil,
			})
		}
	}

	if len(peers) == 0 || registerSuccess == false {
		// 非常奇怪，还必须得保证在只有一个节点的时候，peers 得加入自身，否则选不出 leader
		peers = append(peers, raft.Peer{
			ID:      m.id,
			Context: nil,
		})
	}

	m.raft = raft.StartNode(m.cfg, peers)
	return m
}

func (m *Moon) sendByRpc(messages []raftpb.Message) {
	for _, message := range messages {
		glog.Infof(raft.DescribeMessage(message, nil))
		glog.Infof("%d send to %v, type %v", m.id, message, message.Type)
		nodeId := node.ID(message.To)
		nodeInfo, err := m.InfoStorage.GetNodeInfo(nodeId)
		if err != nil {
			logger.Warningf("Get Node Info fail: %v", err)
			return
		}
		port := strconv.FormatUint(nodeInfo.RpcPort, 10)
		conn, err := grpc.Dial(nodeInfo.IpAddr+":"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Errorf("faild to connect: %v", err)
			return
		}
		defer func(conn *grpc.ClientConn) {
			err = conn.Close()
			if err != nil {
				logger.Errorf("close grpc conn err: %v", err)
			}
		}(conn)

		c := NewMoonClient(conn)
		_, err = c.SendRaftMessage(context.Background(), &message)
		if err != nil {
			logger.Errorf("could not send raft message: %v", err)
		}
		//logger.Infof("Send raft message to: %v success!\n", nodeId)
	}
}

func (m *Moon) process(entry raftpb.Entry) {
	if entry.Type == raftpb.EntryNormal && entry.Data != nil {
		var nodeInfo node.NodeInfo
		err := json.Unmarshal(entry.Data, &nodeInfo)
		if err != nil {
			logger.Errorf("Moon process nodeInfo err: %v", err.Error())
		}
		logger.Infof("Node %v: get Moon info %v", m.id, nodeInfo)
		_ = m.InfoStorage.UpdateNodeInfo(&nodeInfo)
	}
}

func (m *Moon) Run() {

	go m.reportSelfInfo()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.ticker:
			m.raft.Tick()
		case rd := <-m.raft.Ready():
			// 将HardState，entries写入持久化存储中
			m.storage.Save(rd.HardState, rd.Entries)
			//if !raft.IsEmptySnap(rd.Snapshot) {
			//	// 如果快照数据不为空，也需要保存快照数据到持久化存储中
			//	m.storage.SaveSnap(rd.Snapshot)
			//	m.raftStorage.ApplySnapshot(rd.Snapshot)
			//}
			_ = m.raftStorage.Append(rd.Entries)
			//go n.send(rd.Messages)
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
			glog.Infof("%d got message from %v to %v, type %v", m.id, message.From, message.To, message.Type)
			_ = m.raft.Step(m.ctx, message)
			glog.Infof("%d status: %v", m.id, m.raft.Status().RaftState)
		}
	}
}

func (m *Moon) Stop() {
	m.cancel()
}

func (m *Moon) reportSelfInfo() {

	for m.raft.Status().Lead == 0 {
		time.Sleep(1 * time.Second)
	}

	logger.Infof("join group success, start report self info")
	info := m.selfInfo
	js, _ := json.Marshal(info)
	err := m.raft.Propose(m.ctx, js)
	if err != nil {
		logger.Errorf("report self info err: %v", err.Error())
	}
}
