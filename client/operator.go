package client

import (
	"bytes"
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"encoding/json"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"strconv"
)

type Operator interface {
	Get(key string) (Operator, error)
	Remove(key string) error
	State() (string, error)
	Info() (interface{}, error)
}

type VolumeOperator struct {
	volumeID string
	client   *Client
}

func (v *VolumeOperator) Remove(key string) error {
	//TODO implement me
	panic("implement me")
}

func (v *VolumeOperator) State() (string, error) {
	//TODO implement me
	panic("implement me")
}

func (v *VolumeOperator) Info() (interface{}, error) {
	//TODO implement me
	panic("implement me")
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

func (v *VolumeOperator) CreateBucket(bucketInfo *infos.BucketInfo) error {
	nodeInfo := v.client.clusterInfo.NodesInfo[0]
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return err
	}
	moonClient := moon.NewMoonClient(conn)
	_, err = moonClient.ProposeInfo(context.Background(), &moon.ProposeInfoRequest{
		Operate:  moon.ProposeInfoRequest_ADD,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	return err
}

type BucketOperator struct {
	bucketInfo *infos.BucketInfo
	client     *Client
}

func (b *BucketOperator) getAlayaClient(key string) (alaya.AlayaClient, error) {
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
	return alayaClient, nil
}

func (b *BucketOperator) Remove(key string) error {
	alayaClient, err := b.getAlayaClient(key)
	if err != nil {
		return err
	}
	_, err = alayaClient.DeleteMeta(context.Background(), &alaya.DeleteMetaRequest{
		ObjId: object.GenObjectId(b.bucketInfo, key),
	})
	return err
}

func (b *BucketOperator) State() (string, error) {
	return protoToJson(b.bucketInfo)
}

func (b *BucketOperator) Info() (interface{}, error) {
	return b.bucketInfo, nil
}

func (b *BucketOperator) Get(key string) (Operator, error) {
	alayaClient, err := b.getAlayaClient(key)
	if err != nil {
		return nil, err
	}
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

func (o *ObjectOperator) Remove(key string) error {
	panic("implement me")
}

func (o *ObjectOperator) State() (string, error) {
	return protoToJson(o.meta)
}

func protoToJson(pb proto.Message) (string, error) {
	// convert proto to json
	marshaller := jsonpb.Marshaler{}
	jsonData, err := marshaller.MarshalToString(pb)
	if err != nil {
		return "marshal data error", err
	}
	var pretty bytes.Buffer
	err = json.Indent(&pretty, []byte(jsonData), "", "  ")
	if err != nil {
		return "indent data error", err
	}
	return pretty.String(), nil
}

func (o *ObjectOperator) Info() (interface{}, error) {
	return o.meta, nil
}
