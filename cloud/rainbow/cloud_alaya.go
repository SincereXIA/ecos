package rainbow

import (
	"context"
	"ecos/cloud/config"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/shared/alaya"
	"ecos/utils/logger"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"path"
	"runtime"
	"strconv"
)

type CloudAlaya struct {
	alaya.UnimplementedAlayaServer
	ctx  context.Context
	conf *config.CloudConfig

	storage alaya.MetaStorage
	r       *Rainbow

	syncEdgeChan chan *object.ObjectMeta
}

func NewCloudAlaya(ctx context.Context, server *messenger.RpcServer, conf *config.CloudConfig, r *Rainbow) *CloudAlaya {
	a := &CloudAlaya{
		ctx:          ctx,
		conf:         conf,
		storage:      alaya.NewMemoryMetaStorage(),
		r:            r,
		syncEdgeChan: make(chan *object.ObjectMeta, 100),
	}
	alaya.RegisterAlayaServer(server, a)
	go a.syncLoop()
	return a
}

func (a *CloudAlaya) RecordObjectMeta(ctx context.Context, meta *object.ObjectMeta) (*common.Result, error) {
	logger.Infof("cloud Record meta: %v", meta)
	meta.ObjId = object.CleanObjectKey(meta.ObjId)
	err := a.storage.RecordMeta(meta)
	if err != nil {
		return nil, err
	}

	// 通知边缘端
	a.syncEdgeChan <- meta

	return &common.Result{
		Status: common.Result_OK,
	}, nil
}

func (a *CloudAlaya) syncLoop() {
	for {
		select {
		case <-a.ctx.Done():
			logger.Infof("cloud alaya sync loop exit")
			return
		case meta := <-a.syncEdgeChan:
			respChan, err := a.r.SendRequestDirect(&Request{
				Method:   Request_PUT,
				Resource: Request_META,
				Meta:     meta,
			})
			if err != nil {
				logger.Errorf("Send meta to edge failed: %v", err)
				a.syncEdgeChan <- meta
				runtime.Gosched()
			}
			_ = <-respChan
			logger.Infof("Send meta: %v to edge success", meta.ObjId)
		}
	}
}

func (a *CloudAlaya) GetObjectMeta(ctx context.Context, req *alaya.MetaRequest) (*object.ObjectMeta, error) {
	meta, err := a.storage.GetMeta(req.ObjId)
	if err != nil {
		logger.Infof("Get meta from cloud failed: %v, try get from edge", err)
		resp, err := a.r.SendRequestDirect(&Request{
			Method:    Request_GET,
			Resource:  Request_META,
			RequestId: req.ObjId,
		})
		if err != nil {
			logger.Errorf("Get meta from edge failed: %v", err)
			return nil, err
		}
		r := <-resp
		if r.Result.Status != common.Result_OK || len(r.Metas) == 0 {
			logger.Errorf("Get meta from edge failed: %v", r.Result.Message)
			return nil, status.Errorf(codes.NotFound, "meta not found")
		}
		meta = r.Metas[0]
		meta.Position = object.ObjectMeta_POSITION_EDGE_CLOUD
		go func() { // save to cloud
			_, err := a.RecordObjectMeta(a.ctx, meta)
			if err != nil {
				logger.Errorf("Record meta to cloud failed: %v", err)
			}
		}()
	}
	return meta, nil
}

func (a *CloudAlaya) ListMeta(ctx context.Context, req *alaya.ListMetaRequest) (*alaya.ObjectMetaList, error) {
	logger.Infof("cloud receive list meta request: %v", req.Prefix)
	// 获取 bucketInfo，用于判断 keySlot 数量
	_, bucketID, key, err := object.SplitPrefixWithoutSlotID(req.Prefix)
	if err != nil {
		return nil, err
	}
	var metas []*object.ObjectMeta
	info, err := a.r.moon.GetInfoDirect(infos.InfoType_BUCKET_INFO, bucketID)
	if err != nil {
		return nil, err
	}
	bucketInfo := info.BaseInfo().GetBucketInfo()

	// 获取所有 keySlot 中满足的 meta
	for i := 1; i <= int(bucketInfo.Config.KeySlotNum); i++ {
		prefix := path.Join(bucketID, strconv.Itoa(i), key)
		prefix = object.CleanObjectKey(prefix)
		ms, err := a.storage.List(prefix)
		if err != nil {
			return nil, err
		}
		metas = append(metas, ms...)
	}

	// 获取边缘端 meta
	resp, err := a.r.SendRequestDirect(&Request{
		Method:    Request_LIST,
		Resource:  Request_META,
		RequestId: req.Prefix,
	})
	if err != nil {
		return nil, err
	}
	// 去重
	totalMetas := make(map[string]*object.ObjectMeta)
	for _, meta := range metas {
		totalMetas[meta.ObjId] = meta
	}
	for r := range resp {
		if r.Result.Status != common.Result_OK {
			return nil, errors.New(r.Result.Message)
		}
		for _, meta := range r.Metas {
			totalMetas[meta.ObjId] = meta
		}
		if r.IsLast {
			break
		}
	}

	var result []*object.ObjectMeta
	for _, meta := range totalMetas {
		result = append(result, meta)
	}

	return &alaya.ObjectMetaList{
		Metas: result,
	}, nil
}

func (a *CloudAlaya) DeleteMeta(context.Context, *alaya.DeleteMetaRequest) (*common.Result, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteMeta not implemented")
}

func (a *CloudAlaya) doDelete(objID string) error {
	return a.storage.Delete(objID)
}

func (a *CloudAlaya) SendRaftMessage(context.Context, *alaya.PGRaftMessage) (*alaya.PGRaftMessage, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendRaftMessage not implemented")
}
