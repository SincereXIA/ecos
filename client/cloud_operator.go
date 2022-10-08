package client

import (
	"context"
	"ecos/cloud/rainbow"
	"ecos/messenger/common"
	"errors"
	"io"
)

type CloudVolumeOperator struct {
	volumeID string
	client   *Client

	ctx context.Context
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
	rainbowClient, err := cbo.client.GetRainbow()
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
			operators = append(operators, &ObjectOperator{meta: meta})
		}

		if resp.IsLast {
			break
		}
	}

	return operators, nil
}
