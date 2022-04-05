package moon

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/messenger"
	"github.com/golang/mock/gomock"
)

type State struct {
	InfoStorageRegister *infos.StorageRegister
	SelfInfo            *infos.NodeInfo
	SelfIsLeader        bool
}

func (state *State) Run() {
	_ = state.InfoStorageRegister.Update(state.SelfInfo)
}

func (state *State) IsLeader() bool {
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

func (state *State) Set(selfInfo, leaderInfo *infos.NodeInfo, peersInfo []*infos.NodeInfo) {
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
	m.EXPECT().Run().DoAndReturn(state.Run).AnyTimes()
	m.EXPECT().GetInfo(gomock.Any(), gomock.Any()).DoAndReturn(state.GetInfo).AnyTimes()
	m.EXPECT().IsLeader().DoAndReturn(state.IsLeader).AnyTimes()
	m.EXPECT().GetInfoDirect(gomock.Any(), gomock.Any()).DoAndReturn(state.GetInfoDirect).AnyTimes()
	m.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(state.Set).AnyTimes()
	m.EXPECT().ProposeConfChangeAddNode(gomock.Any(), gomock.Any()).AnyTimes()
	m.EXPECT().GetLeaderID().DoAndReturn(state.GetLeaderID).AnyTimes()
	RegisterMoonServer(rpcServer, m)
}
