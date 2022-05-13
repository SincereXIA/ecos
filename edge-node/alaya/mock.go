package alaya

import (
	"context"
	"ecos/edge-node/object"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"github.com/golang/mock/gomock"
)

type MockState struct {
	storage MetaStorage
	UnimplementedAlayaServer
}

func (state *MockState) GetObjectMeta(_ context.Context, req *MetaRequest) (*object.ObjectMeta, error) {
	req.ObjId = object.CleanObjectKey(req.ObjId)
	return state.storage.GetMeta(req.ObjId)
}

func (state *MockState) ListMeta(_ context.Context, req *ListMetaRequest) (*ObjectMetaList, error) {
	req.Prefix = object.CleanObjectKey(req.Prefix)
	metas, _ := state.storage.List(req.Prefix)
	return &ObjectMetaList{
		Metas: metas,
	}, nil
}

func (state *MockState) DeleteMeta(_ context.Context, req *DeleteMetaRequest) (*common.Result, error) {
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

func (state *MockState) SendRaftMessage(context.Context, *PGRaftMessage) (*PGRaftMessage, error) {
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

func InitMock(alaya *MockAlayaer, rpcServer *messenger.RpcServer,
	storage MetaStorage) {
	state := &MockState{storage: storage}
	alaya.EXPECT().Run().Times(1)
	alaya.EXPECT().GetObjectMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.GetObjectMeta).AnyTimes()
	alaya.EXPECT().ListMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.ListMeta).AnyTimes()
	alaya.EXPECT().DeleteMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.DeleteMeta).AnyTimes()
	alaya.EXPECT().RecordObjectMeta(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.RecordObjectMeta).AnyTimes()
	alaya.EXPECT().SendRaftMessage(gomock.Any(), gomock.Not(nil)).DoAndReturn(state.SendRaftMessage).AnyTimes()
	alaya.EXPECT().IsAllPipelinesOK().Return(true).AnyTimes()
	alaya.EXPECT().IsChanged().DoAndReturn(state.IsChanged).AnyTimes()
	alaya.EXPECT().GetReports().DoAndReturn(state.GetReports).AnyTimes()
	alaya.EXPECT().Stop().MaxTimes(1)

	RegisterAlayaServer(rpcServer, alaya)
}
