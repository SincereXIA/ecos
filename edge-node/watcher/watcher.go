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
	selfNodeInfo *infos.NodeInfo
	moon         *moon.Moon
	register     *infos.StorageRegister
	timer        *time.Timer
	config       *Config
	ctx          context.Context
	mutex        sync.Mutex

	currentClusterInfo *infos.ClusterInfo

	cancelFunc context.CancelFunc

	UnimplementedWatcherServer
}

// AddNewNodeToCluster will propose a new NodeInfo in moon,
// if success, it will propose a ConfChang, to add the raftNode into moon group
func (w *Watcher) AddNewNodeToCluster(_ context.Context, info *infos.NodeInfo) (*AddNodeReply, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	request := &moon.ProposeInfoRequest{
		Head: &common.Head{
			Timestamp: timestamp.Now(),
			Term:      w.getCurrentTerm(),
		},
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       strconv.FormatUint(info.RaftId, 10),
		BaseInfo: &infos.BaseInfo{Info: &infos.BaseInfo_NodeInfo{NodeInfo: info}},
	}
	_, err := w.moon.ProposeInfo(w.ctx, request)
	if err != nil {
		// TODO
		return nil, err
	}
	err = w.moon.ProposeConfChangeAddNode(w.ctx, info)
	if err != nil {
		// TODO
		return nil, err
	}
	return &AddNodeReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		PeersNodeInfo: w.getCurrentPeerInfo(),
		LeaderInfo:    nil,
	}, nil
}

func (w *Watcher) GetClusterInfo(_ context.Context, request *GetClusterInfoRequest) (*GetClusterInfoReply, error) {
	if request.Term == 0 {
		return &GetClusterInfoReply{
			Result: &common.Result{
				Status: common.Result_OK,
			},
			ClusterInfo: w.GetCurrentClusterInfo(),
		}, nil
	}
	clusterInfoStorage := w.register.GetStorage(infos.InfoType_CLUSTER_INFO)
	info, err := clusterInfoStorage.Get(strconv.FormatUint(request.Term, 10))
	if err != nil {
		// TODO
		return nil, err
	}
	return &GetClusterInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		ClusterInfo: info.BaseInfo().GetClusterInfo(),
	}, nil
}

func (w *Watcher) GetCurrentClusterInfo() *infos.ClusterInfo {
	return w.currentClusterInfo
}

func (w *Watcher) getCurrentTerm() uint64 {
	if w.currentClusterInfo != nil {
		return w.currentClusterInfo.Term
	}
	return 0
}

func (w *Watcher) getCurrentPeerInfo() []*infos.NodeInfo {
	nodeInfoStorage := w.register.GetStorage(infos.InfoType_NODE_INFO)
	nodeInfos, err := nodeInfoStorage.GetAll()
	if err != nil {
		// TODO
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
		// TODO
	}
	var clusterNodes []*infos.NodeInfo
	for _, info := range nodeInfos {
		clusterNodes = append(clusterNodes, info.BaseInfo().GetNodeInfo())
	}
	sort.Slice(clusterNodes, func(i, j int) bool {
		return clusterNodes[i].RaftId > clusterNodes[j].RaftId
	})
	leaderID := w.moon.GetLeaderID()
	// TODO (zhang): need a way to gen infoStorage key
	leaderInfo, err := nodeInfoStorage.Get(strconv.FormatUint(leaderID, 10))
	return &infos.ClusterInfo{
		Term:            uint64(time.Now().UnixNano()),
		LeaderInfo:      leaderInfo.BaseInfo().GetNodeInfo(),
		NodesInfo:       clusterNodes,
		UpdateTimestamp: nil,
	}
}

func (w *Watcher) RequestJoinCluster(leaderInfo *infos.NodeInfo) error {
	if leaderInfo == nil || leaderInfo.RaftId == w.selfNodeInfo.RaftId {
		// 此节点为集群中第一个节点
		w.moon.Init(leaderInfo, nil)
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
	w.moon.Init(leaderInfo, reply.PeersNodeInfo)
	return nil
}

func (w *Watcher) StartMoon() {
	go w.moon.Run()
}

func (w *Watcher) AskSky() (leaderInfo *infos.NodeInfo, err error) {
	if w.config.SunAddr == "" {
		return nil, errno.ConnectSunFail
	}
	conn, err := grpc.Dial(w.config.SunAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	c := sun.NewSunClient(conn)
	result, err := c.MoonRegister(context.Background(), w.selfNodeInfo)
	if err != nil {
		return nil, err
	}
	// update raftID for watcher and moon
	w.selfNodeInfo.RaftId = result.RaftId
	w.moon.SelfInfo = w.selfNodeInfo
	return result.ClusterInfo.LeaderInfo, nil
}

func NewWatcher(ctx context.Context, config *Config, server *messenger.RpcServer,
	m *moon.Moon, register *infos.StorageRegister) *Watcher {
	watcherCtx, cancelFunc := context.WithCancel(ctx)
	watcher := &Watcher{
		moon:         m,
		register:     register,
		config:       config,
		selfNodeInfo: &config.SelfNodeInfo,
		ctx:          watcherCtx,
		cancelFunc:   cancelFunc,
	}
	nodeInfoStorage := watcher.register.GetStorage(infos.InfoType_NODE_INFO)
	clusterInfoStorage := watcher.register.GetStorage(infos.InfoType_CLUSTER_INFO)
	nodeInfoStorage.SetOnUpdate(func(info infos.Information) {
		if !watcher.moon.IsLeader() {
			return
		}
		if watcher.timer != nil && watcher.timer.Stop() {
			watcher.timer.Reset(watcher.config.NodeInfoCommitInterval)
			return
		}
		watcher.timer = time.AfterFunc(watcher.config.NodeInfoCommitInterval, func() {
			clusterInfo := watcher.genNewClusterInfo()
			request := &moon.ProposeInfoRequest{
				Head: &common.Head{
					Timestamp: timestamp.Now(),
					Term:      watcher.getCurrentTerm(),
				},
				Operate: moon.ProposeInfoRequest_ADD,
				Id:      strconv.FormatUint(clusterInfo.Term, 10),
				BaseInfo: &infos.BaseInfo{Info: &infos.BaseInfo_ClusterInfo{
					ClusterInfo: clusterInfo,
				}},
			}
			logger.Infof("[NEW TERM] leader: %v propose new cluster info, term: %v, node num: %v",
				watcher.selfNodeInfo.RaftId, request.BaseInfo.GetClusterInfo().Term,
				len(request.BaseInfo.GetClusterInfo().NodesInfo))
			_, err := watcher.moon.ProposeInfo(watcher.ctx, request)
			if err != nil {
				// TODO
				return
			}
			logger.Infof("[NEW TERM] leader propose new cluster info success")
		})
	})
	clusterInfoStorage.SetOnUpdate(func(info infos.Information) {
		logger.Infof("[NEW TERM] node %v get new term: %v, node num: %v",
			watcher.selfNodeInfo.RaftId, info.BaseInfo().GetClusterInfo().Term,
			len(info.BaseInfo().GetClusterInfo().NodesInfo))
		watcher.currentClusterInfo = info.BaseInfo().GetClusterInfo()
	})
	RegisterWatcherServer(server, watcher)
	return watcher
}
