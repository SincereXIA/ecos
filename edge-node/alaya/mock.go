package alaya

import (
	"context"
	"ecos/edge-node/object"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/alaya"
	"github.com/golang/mock/gomock"
)

type MockState struct {
	storage alaya.MetaStorage
	alaya.UnimplementedAlayaServer
}

func (state *MockState) GetObjectMeta(_ context.Context, req *alaya.MetaRequest) (*object.ObjectMeta, error) {
	req.ObjId = object.CleanObjectKey(req.ObjId)
	return state.storage.GetMeta(req.ObjId)
}

func (state *MockState) ListMeta(_ context.Context, req *alaya.ListMetaRequest) (*alaya.ObjectMetaList, error) {
	req.Prefix = object.CleanObjectKey(req.Prefix)
	metas, _ := state.storage.List(req.Prefix)
	return &alaya.ObjectMetaList{
		Metas: metas,
	}, nil
}

func (state *MockState) DeleteMeta(_ context.Context, req *alaya.DeleteMetaRequest) (*common.Result, error) {
	req.ObjId = object.CleanObjectKey(req.ObjId)
	err := state.storage.Delete(req.ObjId)
	if err != nil {
		return nil, err
	}
	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (state *MockState) RecordObjectMeta(_ context.Context, meta *object.ObjectMeta) (*common.Result, error) {
	meta.ObjId = object.CleanObjectKey(meta.ObjId)
	err := state.storage.RecordMeta(meta)
	if err != nil {
		return nil, err
	}
	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (state *MockState) SendRaftMessage(context.Context, *alaya.PGRaftMessage) (*alaya.PGRaftMessage, error) {
	return nil, nil
}

func (state *MockState) IsAllPipelinesOK() bool {
	return true
}

func (state *MockState) IsChanged() bool {
	return true
}

func (state *MockState) GetReports() []watcher.Report {
	return nil
}

func InitMock(alayaer *MockAlayaer, rpcServer *messenger.RpcServer,
	storage alaya.MetaStorage) {
	state := &MockState{storage: storage}
	alayaer.EXPECT().Run().Times(1)
	alayaer.EXPECT().GetObjectMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.GetObjectMeta).AnyTimes()
	alayaer.EXPECT().ListMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.ListMeta).AnyTimes()
	alayaer.EXPECT().DeleteMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.DeleteMeta).AnyTimes()
	alayaer.EXPECT().RecordObjectMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.RecordObjectMeta).AnyTimes()
	alayaer.EXPECT().SendRaftMessage(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.SendRaftMessage).AnyTimes()
	alayaer.EXPECT().IsAllPipelinesOK().Return(true).AnyTimes()
	alayaer.EXPECT().IsChanged().DoAndReturn(state.IsChanged).AnyTimes()
	alayaer.EXPECT().GetReports().DoAndReturn(state.GetReports).AnyTimes()
	alayaer.EXPECT().Stop().MaxTimes(1)

	alaya.RegisterAlayaServer(rpcServer, alayaer)
}
