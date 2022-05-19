package moon

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"ecos/messenger/common"
	"github.com/golang/mock/gomock"
	"sync"
)

type State struct {
	InfoStorageRegister *infos.StorageRegister
	SelfInfo            *infos.NodeInfo
	SelfIsLeader        bool
	mutex               sync.RWMutex
}

func (state *State) Run() {
	_ = state.InfoStorageRegister.Update(state.SelfInfo)
}

func (state *State) IsLeader() bool {
	state.mutex.RLock()
	defer state.mutex.RUnlock()
	return state.SelfIsLeader
}

func (state *State) GetInfoDirect(infoType infos.InfoType, id string) (infos.Information, error) {
	return state.InfoStorageRegister.Get(infoType, id)
}

func (state *State) ProposeInfo(_ context.Context, req *ProposeInfoRequest) (*ProposeInfoReply, error) {
	_ = state.InfoStorageRegister.Update(req.BaseInfo)
	return &ProposeInfoReply{}, nil
}

func (state *State) GetInfo(_ context.Context, req *GetInfoRequest) (*GetInfoReply, error) {
	infoType := req.InfoType
	id := req.InfoId
	info, err := state.GetInfoDirect(infoType, id)
	if err != nil {
		return nil, err
	}
	return &GetInfoReply{
		BaseInfo: info.BaseInfo(),
	}, nil
}

func (state *State) ListInfo(_ context.Context, req *ListInfoRequest) (*ListInfoReply, error) {
	infoType := req.InfoType
	result, err := state.InfoStorageRegister.List(infoType, req.Prefix)
	if err != nil {
		return nil, err
	}
	baseInfos := make([]*infos.BaseInfo, 0, len(result))
	for _, info := range result {
		baseInfos = append(baseInfos, info.BaseInfo())
	}
	return &ListInfoReply{
		Result: &common.Result{
			Status: common.Result_OK,
		},
		BaseInfos: baseInfos,
	}, nil
}

func (state *State) Set(selfInfo, leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo) {
	state.mutex.Lock()
	defer state.mutex.Unlock()
	state.SelfInfo = selfInfo
	state.SelfIsLeader = leaderInfo != nil && leaderInfo.RaftId == selfInfo.RaftId
}

func (state *State) GetLeaderID() uint64 {
	return 1
}

func InitMock(m *MockInfoController, rpcServer *messenger.RpcServer,
	register *infos.StorageRegister, selfInfo *infos.NodeInfo, selfIsLeader bool) {
	state := &State{
		InfoStorageRegister: register,
		SelfInfo:            selfInfo,
		SelfIsLeader:        selfIsLeader,
	}
	m.EXPECT().ProposeInfo(gomock.Any(), gomock.Any()).DoAndReturn(state.ProposeInfo).AnyTimes()
	m.EXPECT().Run().DoAndReturn(state.Run).Times(1)
	m.EXPECT().GetInfo(gomock.Any(), gomock.Any()).DoAndReturn(state.GetInfo).AnyTimes()
	m.EXPECT().IsLeader().DoAndReturn(state.IsLeader).AnyTimes()
	m.EXPECT().GetInfoDirect(gomock.Any(), gomock.Any()).DoAndReturn(state.GetInfoDirect).AnyTimes()
	m.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(state.Set).AnyTimes()
	m.EXPECT().ProposeConfChangeAddNode(gomock.Any(), gomock.Any()).AnyTimes()
	m.EXPECT().GetLeaderID().DoAndReturn(state.GetLeaderID).AnyTimes()
	m.EXPECT().Stop().AnyTimes()
	m.EXPECT().ListInfo(gomock.Any(), gomock.Any()).DoAndReturn(state.ListInfo).AnyTimes()
	RegisterMoonServer(rpcServer, m)
}
