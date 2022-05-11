package watcher

import (
	"context"
	"ecos/cloud/sun"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"ecos/utils/timestamp"
	"github.com/mohae/deepcopy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Watcher process edge-node join/leave cluster & maintain ClusterInfo.
//
// Night watcher swear：
//   Night gathers, and now my watch begins. It shall not end until my death.
//   I shall take no wife, hold no lands, father no children.
//   I shall wear no crowns and win no glory.
//   I shall live and die at my post.
//   I am the sword in the darkness.
//   I am the watcher on the walls.
//   I am the fire that burns against the cold, the light that brings the dawn,
//   the horn that wakes the sleepers, the shield that guards the realms of men.
//   I pledge my life and honor to the Night’s Watch,
//   for this night and all the nights to come.
type Watcher struct {
	ctx          context.Context
	selfNodeInfo *infos.NodeInfo
	moon         moon.InfoController
	Monitor      Monitor
	register     *infos.StorageRegister
	timer        *time.Timer
	timerMutex   sync.Mutex
	config       *Config

	addNodeMutex sync.Mutex

	currentClusterInfo infos.ClusterInfo
	clusterInfoMutex   sync.RWMutex

	cancelFunc context.CancelFunc

	UnimplementedWatcherServer
}

// AddNewNodeToCluster will propose a new NodeInfo in moon,
// if success, it will propose a ConfChang, to add the raftNode into moon group
func (w *Watcher) AddNewNodeToCluster(_ context.Context, info *infos.NodeInfo) (*AddNodeReply, error) {
	w.addNodeMutex.Lock()
	defer w.addNodeMutex.Unlock()

	flag := true

	currentPeerInfos := w.getCurrentPeerInfo()
	for _, peerInfo := range currentPeerInfos {
		if peerInfo.RaftId == info.RaftId {
			flag = false
			w.moon.NodeInfoChanged(info)
			break
		}
	}

	if flag {
		request := &moon.ProposeInfoRequest{
			Head: &common.Head{
				Timestamp: timestamp.Now(),
				Term:      w.GetCurrentTerm(),
			},
			Operate:  moon.ProposeInfoRequest_ADD,
			Id:       strconv.FormatUint(info.RaftId, 10),
			BaseInfo: &infos.BaseInfo{Info: &infos.BaseInfo_NodeInfo{NodeInfo: info}},
		}
		_, err := w.moon.ProposeInfo(w.ctx, request)
		logger.Infof("propose New nodeInfo: %v", info.RaftId)
		if err != nil {
			// TODO
			return nil, err
		}
		err = w.moon.ProposeConfChangeAddNode(w.ctx, info)
		logger.Infof("propose conf change to add node: %v", info.RaftId)
		if err != nil {
			// TODO
			return nil, err
		}
	}

	return &AddNodeReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		PeersNodeInfo: w.getCurrentPeerInfo(),
		LeaderInfo:    nil,
	}, nil
}

// GetClusterInfo return requested cluster info to rpc client,
// if GetClusterInfoRequest.Term == 0, it will return current cluster info.
func (w *Watcher) GetClusterInfo(_ context.Context, request *GetClusterInfoRequest) (*GetClusterInfoReply, error) {
	info, err := w.GetClusterInfoByTerm(request.Term)
	if err != nil {
		return &GetClusterInfoReply{
			Result: &common.Result{
				Status:  common.Result_FAIL,
				Message: err.Error(),
			},
			ClusterInfo: nil,
		}, err
	}
	return &GetClusterInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		ClusterInfo: &info,
	}, nil
}

// GetClusterInfoByTerm return cluster info directly, it is called by other components
// on same node.
func (w *Watcher) GetClusterInfoByTerm(term uint64) (infos.ClusterInfo, error) {
	if term == 0 {
		info := w.GetCurrentClusterInfo()
		if info.Term == 0 {
			return info, errno.InfoNotFound
		}
		return info, nil
	}
	clusterInfoStorage := w.register.GetStorage(infos.InfoType_CLUSTER_INFO)
	info, err := clusterInfoStorage.Get(strconv.FormatUint(term, 10))
	if err != nil {
		return infos.ClusterInfo{}, err
	}
	return *info.BaseInfo().GetClusterInfo(), nil
}

func (w *Watcher) GetCurrentClusterInfo() infos.ClusterInfo {
	w.clusterInfoMutex.RLock()
	defer w.clusterInfoMutex.RUnlock()
	return w.currentClusterInfo
}

func (w *Watcher) SetOnInfoUpdate(infoType infos.InfoType, name string, f infos.StorageUpdateFunc) error {
	storage := w.register.GetStorage(infoType)
	storage.SetOnUpdate(name, f)
	return nil
}

func (w *Watcher) GetCurrentTerm() uint64 {
	return w.GetCurrentClusterInfo().Term
}

func (w *Watcher) getCurrentPeerInfo() []*infos.NodeInfo {
	nodeInfoStorage := w.register.GetStorage(infos.InfoType_NODE_INFO)
	nodeInfos, err := nodeInfoStorage.GetAll()
	if err != nil {
		logger.Errorf("get nodeInfo from nodeInfoStorage fail: %v", err)
		return nil
	}
	var peerNodes []*infos.NodeInfo
	for _, info := range nodeInfos {
		peerNodes = append(peerNodes, info.BaseInfo().GetNodeInfo())
	}
	sort.Slice(peerNodes, func(i, j int) bool {
		return peerNodes[i].RaftId > peerNodes[j].RaftId
	})
	return peerNodes
}

func (w *Watcher) genNewClusterInfo() *infos.ClusterInfo {
	nodeInfoStorage := w.register.GetStorage(infos.InfoType_NODE_INFO)
	nodeInfos, err := nodeInfoStorage.GetAll()
	if err != nil {
		logger.Errorf("get nodeInfo from nodeInfoStorage fail: %v", err)
		return nil
	}
	var clusterNodes []*infos.NodeInfo
	for _, info := range nodeInfos {
		// copy before change to avoid data race
		nodeInfo := deepcopy.Copy(info.BaseInfo().GetNodeInfo()).(*infos.NodeInfo)
		report := w.Monitor.GetNodeReport(nodeInfo.RaftId)
		if report == nil {
			logger.Warningf("get report: %v from Monitor fail", nodeInfo.RaftId)
			logger.Warningf("set node: %v state OFFLINE", nodeInfo.RaftId)
			nodeInfo.State = infos.NodeState_OFFLINE
		} else {
			nodeInfo.State = report.State
		}
		clusterNodes = append(clusterNodes, nodeInfo)
	}
	sort.Slice(clusterNodes, func(i, j int) bool {
		return clusterNodes[i].RaftId > clusterNodes[j].RaftId
	})
	leaderID := w.moon.GetLeaderID()
	// TODO (zhang): need a way to gen infoStorage key
	leaderInfo, err := nodeInfoStorage.Get(strconv.FormatUint(leaderID, 10))
	if err != nil {
		logger.Errorf("get leaderInfo from nodeInfoStorage fail: %v", err)
		return nil
	}
	return &infos.ClusterInfo{
		Term:            uint64(time.Now().UnixNano()),
		LeaderInfo:      leaderInfo.BaseInfo().GetNodeInfo(),
		NodesInfo:       clusterNodes,
		UpdateTimestamp: nil,
		MetaPgNum:       10,
		MetaPgSize:      3,
		BlockPgNum:      100,
		BlockPgSize:     3,
		LastTerm:        w.GetCurrentTerm(),
	}
}

func (w *Watcher) RequestJoinCluster(leaderInfo *infos.NodeInfo) error {
	if leaderInfo == nil || leaderInfo.RaftId == w.selfNodeInfo.RaftId {
		// 此节点为集群中第一个节点
		w.moon.Set(w.selfNodeInfo, leaderInfo, nil)
		return nil
	}

	conn, err := messenger.GetRpcConnByNodeInfo(leaderInfo)
	if err != nil {
		logger.Errorf("Request Join group err: %v", err.Error())
		return err
	}
	client := NewWatcherClient(conn)
	reply, err := client.AddNewNodeToCluster(w.ctx, w.selfNodeInfo)
	if err != nil {
		logger.Errorf("Request join to group err: %v", err.Error())
		return err
	}
	w.moon.Set(w.selfNodeInfo, leaderInfo, reply.PeersNodeInfo)
	return nil
}

func (w *Watcher) StartMoon() {
	go w.moon.Run()
}

func (w *Watcher) GetMoon() moon.InfoController {
	return w.moon
}

func (w *Watcher) AskSky() (leaderInfo *infos.NodeInfo, err error) {
	if w.config.SunAddr == "" {
		return nil, errno.ConnectSunFail
	}
	conn, err := grpc.Dial(w.config.SunAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	c := sun.NewSunClient(conn)
	result, err := c.MoonRegister(context.Background(), w.selfNodeInfo)
	if err != nil {
		return nil, err
	}
	// update raftID for watcher and moon
	w.selfNodeInfo.RaftId = result.RaftId
	return result.ClusterInfo.LeaderInfo, nil
}

func (w *Watcher) GetSelfInfo() *infos.NodeInfo {
	return w.selfNodeInfo
}

func (w *Watcher) Run() {
	// start Monitor
	go w.Monitor.Run()
	// watch Monitor
	go w.processMonitor()
	leaderInfo, err := w.AskSky()
	if err != nil {
		logger.Warningf("watcher ask sky err: %v", err)
	} else {
		err = w.RequestJoinCluster(leaderInfo)
		if err != nil {
			logger.Errorf("watcher request join to cluster err: %v", err)
		}
	}
	logger.Infof("%v request join to cluster success", w.GetSelfInfo().RaftId)
	w.StartMoon()
	logger.Infof("moon init success, NodeID: %v", w.GetSelfInfo().RaftId)
}

func (w *Watcher) processMonitor() {
	c := w.Monitor.GetEventChannel()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-c:
			logger.Warningf("watcher receive Monitor event")
			w.nodeInfoChanged(nil)
		}
	}
}

func (w *Watcher) initCluster() {
	rootDefaultBucket := infos.GenBucketInfo("root", "default", "root")
	_, err := w.moon.GetInfoDirect(infos.InfoType_BUCKET_INFO, rootDefaultBucket.GetID())
	if err == nil { // root default bucket exist
		return
	}
	logger.Infof("init root default bucket")
	_, err = w.moon.ProposeInfo(w.ctx, &moon.ProposeInfoRequest{
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       rootDefaultBucket.GetID(),
		BaseInfo: rootDefaultBucket.BaseInfo(),
	})
	if err != nil {
		logger.Errorf("init cluster fail: %v", err)
	}
}

func (w *Watcher) proposeClusterInfo(clusterInfo *infos.ClusterInfo) {
	if !w.moon.IsLeader() {
		return
	}
	request := &moon.ProposeInfoRequest{
		Head: &common.Head{
			Timestamp: timestamp.Now(),
			Term:      w.GetCurrentTerm(),
		},
		Operate: moon.ProposeInfoRequest_ADD,
		Id:      strconv.FormatUint(clusterInfo.Term, 10),
		BaseInfo: &infos.BaseInfo{Info: &infos.BaseInfo_ClusterInfo{
			ClusterInfo: clusterInfo,
		}},
	}
	logger.Infof("[NEW TERM] leader: %v propose new cluster info, term: %v, node num: %v",
		w.selfNodeInfo.RaftId, request.BaseInfo.GetClusterInfo().Term,
		len(request.BaseInfo.GetClusterInfo().NodesInfo))
	_, err := w.moon.ProposeInfo(w.ctx, request)
	if err != nil {
		// TODO
		return
	}
	logger.Infof("[NEW TERM] leader propose new cluster info success")
}

func (w *Watcher) nodeInfoChanged(_ infos.Information) {
	w.timerMutex.Lock()
	defer w.timerMutex.Unlock()
	if !w.moon.IsLeader() {
		return
	}
	if w.timer != nil && w.timer.Stop() {
		w.timer.Reset(w.config.NodeInfoCommitInterval)
		return
	}
	w.timer = time.AfterFunc(w.config.NodeInfoCommitInterval, func() {
		clusterInfo := w.genNewClusterInfo()
		if clusterInfo == nil {
			return
		}
		w.proposeClusterInfo(clusterInfo)
	})
}

func (w *Watcher) clusterInfoChanged(info infos.Information) {
	if w.currentClusterInfo.Term == uint64(0) && w.moon.IsLeader() { // first time
		go w.initCluster()
	}
	w.clusterInfoMutex.Lock()
	defer w.clusterInfoMutex.Unlock()
	logger.Infof("[NEW TERM] node %v get new term: %v, node num: %v",
		w.selfNodeInfo.RaftId, info.BaseInfo().GetClusterInfo().Term,
		len(info.BaseInfo().GetClusterInfo().NodesInfo))
	w.currentClusterInfo = *info.BaseInfo().GetClusterInfo()
}

func NewWatcher(ctx context.Context, config *Config, server *messenger.RpcServer,
	m moon.InfoController, register *infos.StorageRegister) *Watcher {
	watcherCtx, cancelFunc := context.WithCancel(ctx)

	watcher := &Watcher{
		moon:         m,
		register:     register,
		config:       config,
		selfNodeInfo: &config.SelfNodeInfo,
		ctx:          watcherCtx,
		cancelFunc:   cancelFunc,
	}
	monitor := NewMonitor(watcherCtx, watcher, server)
	watcher.Monitor = monitor
	nodeInfoStorage := watcher.register.GetStorage(infos.InfoType_NODE_INFO)
	clusterInfoStorage := watcher.register.GetStorage(infos.InfoType_CLUSTER_INFO)
	nodeInfoStorage.SetOnUpdate("watcher-"+watcher.selfNodeInfo.Uuid, watcher.nodeInfoChanged)
	clusterInfoStorage.SetOnUpdate("watcher-"+watcher.selfNodeInfo.Uuid, watcher.clusterInfoChanged)
	NewStatusReporter(ctx, watcher)

	RegisterWatcherServer(server, watcher)
	return watcher
}
