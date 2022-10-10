package client

import (
	"context"
	"ecos/cloud/rainbow"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/messenger/common"
	"errors"
	"io"
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
	rainbowClient, err := cbo.client.getRainbow()
	if err != nil {
		return nil, err
	}

	stream, err := rainbowClient.SendRequest(cbo.ctx, &rainbow.Request{
		Method:    rainbow.Request_LIST,
		Resource:  rainbow.Request_META,
		RequestId: prefix,
	})
	if err != nil {
		return nil, err
	}

	var operators []Operator
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if resp.Result.Status != common.Result_OK {
			return nil, errors.New(resp.Result.Message)
		}

		for _, meta := range resp.Metas {
			operators = append(operators, &CloudObjectOperator{meta: meta})
		}

		if resp.IsLast {
			break
		}
	}

	return operators, nil
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
