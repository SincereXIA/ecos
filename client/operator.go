package client

import (
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"github.com/gogo/protobuf/jsonpb"
	"strconv"
)

type Operator interface {
	Get(key string) (Operator, error)
	Remove(key string) (Operator, error)
	State() (string, error)
}

type VolumeOperator struct {
	volumeID string
	client   *Client
}

func (v *VolumeOperator) Get(key string) (Operator, error) {
	nodeInfo := v.client.clusterInfo.NodesInfo[0]
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return nil, err
	}
	moonClient := moon.NewMoonClient(conn)
	info, err := moonClient.GetInfo(context.Background(), &moon.GetInfoRequest{
		InfoType: infos.InfoType_BUCKET_INFO,
		InfoId:   infos.GenBucketID(v.volumeID, key),
	})

	return &BucketOperator{
		bucketInfo: info.BaseInfo.GetBucketInfo(),
		client:     v.client,
	}, nil
}

type BucketOperator struct {
	bucketInfo *infos.BucketInfo
	client     *Client
}

func (b *BucketOperator) Remove(key string) (Operator, error) {
	//TODO implement me
	panic("implement me")
}

func (b *BucketOperator) State() (string, error) {
	//TODO implement me
	panic("implement me")
}

func (b *BucketOperator) Get(key string) (Operator, error) {
	pgID := object.GenObjPgID(b.bucketInfo, key, b.client.clusterInfo.MetaPgNum)
	cp, err := pipeline.NewClusterPipelines(b.client.clusterInfo)
	if err != nil {
		return nil, err
	}

	nodeID := cp.GetMetaPG(pgID)[0]
	info, err := b.client.infoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(nodeID, 10))
	if err != nil {
		return nil, err
	}
	conn, err := messenger.GetRpcConnByNodeInfo(info.BaseInfo().GetNodeInfo())
	if err != nil {
		return nil, err
	}
	alayaClient := alaya.NewAlayaClient(conn)
	reply, err := alayaClient.GetObjectMeta(context.Background(), &alaya.MetaRequest{
		ObjId: object.GenObjectId(b.bucketInfo, key),
	})
	if err != nil {
		return nil, err
	}
	return &ObjectOperator{meta: reply}, nil

}

type ObjectOperator struct {
	meta *object.ObjectMeta
}

func (o *ObjectOperator) Get(key string) (Operator, error) {
	panic("implement me")
}

func (o *ObjectOperator) Remove(key string) (Operator, error) {
	panic("implement me")
}

func (o *ObjectOperator) State() (string, error) {
	// convert proto to json
	marshaller := jsonpb.Marshaler{}
	jsonData, err := marshaller.MarshalToString(o.meta)
	if err != nil {
		return "marshal data error", err
	}
	return jsonData, nil
}
