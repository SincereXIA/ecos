package client

import (
	"context"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
)

type CloudVolumeOperator struct {
	volumeID string
	client   *Client

	ctx context.Context
}

func (cvo *CloudVolumeOperator) List(prefix string) ([]Operator, error) {
	//TODO implement me
	panic("implement me")
}

func (cvo *CloudVolumeOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me")
}

func (cvo *CloudVolumeOperator) State() (string, error) {
	//TODO implement me
	panic("implement me")
}

func (cvo *CloudVolumeOperator) Info() (interface{}, error) {
	//TODO implement me
	panic("implement me")
}

func (cvo *CloudVolumeOperator) CreateBucket(bucketInfo *infos.BucketInfo) error {
	//TODO implement me
	panic("implement me")
}

func (cvo *CloudVolumeOperator) DeleteBucket(bucketInfo *infos.BucketInfo) error {
	//TODO implement me
	panic("implement me")
}

func NewCloudVolumeOperator(ctx context.Context, client *Client, volumeID string) *CloudVolumeOperator {
	return &CloudVolumeOperator{
		volumeID: volumeID,
		client:   client,
		ctx:      ctx,
	}
}

func (cvo *CloudVolumeOperator) Get(key string) (Operator, error) {
	return NewCloudBucketOperator(cvo.ctx, cvo.client, key), nil
}

type CloudBucketOperator struct {
	bucketName string
	client     *Client

	ctx context.Context
}

func (cbo *CloudBucketOperator) Get(key string) (Operator, error) {
	//TODO implement me
	panic("implement me")
}

func (cbo *CloudBucketOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me")
}

func (cbo *CloudBucketOperator) State() (string, error) {
	//TODO implement me
	panic("implement me")
}

func (cbo *CloudBucketOperator) Info() (interface{}, error) {
	//TODO implement me
	panic("implement me")
}

func NewCloudBucketOperator(ctx context.Context, client *Client, bucketName string) *CloudBucketOperator {
	return &CloudBucketOperator{
		bucketName: bucketName,
		client:     client,
		ctx:        ctx,
	}
}

func (cbo *CloudBucketOperator) List(prefix string) ([]Operator, error) {
	metas, err := cbo.client.ListObjects(cbo.ctx, cbo.bucketName, prefix)
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
	panic("implement me")
}

func (c CloudObjectOperator) List(prefix string) ([]Operator, error) {
	//TODO implement me
	panic("implement me")
}

func (c CloudObjectOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me")
}

func (c CloudObjectOperator) State() (string, error) {
	//TODO implement me
	return protoToJson(c.meta)
}

func (c CloudObjectOperator) Info() (interface{}, error) {
	//TODO implement me
	return protoToJson(c.meta)
}

func (coo *CloudObjectOperator) Read(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}
