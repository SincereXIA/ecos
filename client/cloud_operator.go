package client

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/shared/moon"
	"ecos/utils/logger"
)

type CloudVolumeOperator struct {
	volumeID string
	client   *Client

	ctx context.Context
}

func (cvo *CloudVolumeOperator) List(prefix string) ([]Operator, error) {
	moonClient, _, err := cvo.client.GetMoon()
	if err != nil {
		logger.Errorf("get moon client err: %v", err.Error())
		return nil, err
	}
	reply, err := moonClient.ListInfo(cvo.ctx, &moon.ListInfoRequest{
		InfoType: infos.InfoType_BUCKET_INFO,
		Prefix:   infos.GenBucketID(cvo.volumeID, prefix),
	})
	if err != nil {
		return nil, err
	}
	var buckets []Operator
	for _, info := range reply.BaseInfos {
		buckets = append(buckets, &CloudBucketOperator{
			info:   info.GetBucketInfo(),
			client: cvo.client,
		})
	}
	return buckets, nil
}

func (cvo *CloudVolumeOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me CloudVolumeOperator.Remove")
}

func (cvo *CloudVolumeOperator) State() (string, error) {
	//TODO implement me
	panic("implement me CloudVolumeOperator.State")
}

func (cvo *CloudVolumeOperator) Info() (interface{}, error) {
	//TODO implement me
	panic("implement me CloudVolumeOperator.Info")
}

func (cvo *CloudVolumeOperator) CreateBucket(bucketInfo *infos.BucketInfo) error {
	moonClient, _, err := cvo.client.GetMoon()
	if err != nil {
		logger.Errorf("get moon client err: %v", err.Error())
		return err
	}
	_, err = moonClient.ProposeInfo(cvo.ctx, &moon.ProposeInfoRequest{
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	return err
}

func (cvo *CloudVolumeOperator) DeleteBucket(bucketInfo *infos.BucketInfo) error {
	//TODO implement me
	panic("implement me CloudVolumeOperator.DeleteBucket")
}

func NewCloudVolumeOperator(ctx context.Context, client *Client, volumeID string) *CloudVolumeOperator {
	return &CloudVolumeOperator{
		volumeID: volumeID,
		client:   client,
		ctx:      ctx,
	}
}

func (cvo *CloudVolumeOperator) Get(key string) (Operator, error) {
	moonClient, _, err := cvo.client.GetMoon()
	if err != nil {
		logger.Errorf("get moon client err: %v", err.Error())
		return nil, err
	}
	reply, err := moonClient.GetInfo(cvo.ctx, &moon.GetInfoRequest{
		InfoType: infos.InfoType_BUCKET_INFO,
		InfoId:   infos.GenBucketID(cvo.volumeID, key),
	})
	if err != nil {
		return nil, err
	}
	info := reply.BaseInfo.GetBucketInfo()
	return NewCloudBucketOperator(cvo.ctx, cvo.client, info), nil
}

type CloudBucketOperator struct {
	info   *infos.BucketInfo
	client *Client

	ctx context.Context
}

func (cbo *CloudBucketOperator) Get(key string) (Operator, error) {
	//TODO implement me
	panic("implement me CloudBucketOperator.Get")
}

func (cbo *CloudBucketOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me CloudBucketOperator.Remove")
}

func (cbo *CloudBucketOperator) State() (string, error) {
	return protoToJson(cbo.info)
}

func (cbo *CloudBucketOperator) Info() (interface{}, error) {
	return protoToJson(cbo.info)
}

func NewCloudBucketOperator(ctx context.Context, client *Client, bucketInfo *infos.BucketInfo) *CloudBucketOperator {
	return &CloudBucketOperator{
		info:   bucketInfo,
		client: client,
		ctx:    ctx,
	}
}

func (cbo *CloudBucketOperator) List(prefix string) ([]Operator, error) {
	metas, err := cbo.client.ListObjects(cbo.ctx, cbo.info.BucketName, prefix)
	if err != nil {
		return nil, err
	}
	ops := make([]Operator, 0, len(metas))
	for _, meta := range metas {
		ops = append(ops, &CloudObjectOperator{
			meta: meta,
		})
	}
	return ops, nil
}

type CloudObjectOperator struct {
	meta *object.ObjectMeta
}

func (c CloudObjectOperator) Get(key string) (Operator, error) {
	//TODO implement me
	panic("implement me cloudObjectOperator Get")
}

func (c CloudObjectOperator) List(prefix string) ([]Operator, error) {
	//TODO implement me
	panic("implement me cloudObjectOperator List")
}

func (c CloudObjectOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me cloudObjectOperator Remove")
}

func (c CloudObjectOperator) State() (string, error) {
	return protoToJson(c.meta)
}

func (c CloudObjectOperator) Info() (interface{}, error) {
	return protoToJson(c.meta)
}

func (coo *CloudObjectOperator) Read(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me cloudObjectOperator Read")
}
