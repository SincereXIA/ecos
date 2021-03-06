package client

import (
	"bytes"
	"context"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/moon"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"encoding/json"
	"errors"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"sort"
	"strconv"
	"strings"
)

type Operator interface {
	Get(key string) (Operator, error)
	List(prefix string) ([]Operator, error)
	Remove(key string) error
	State() (string, error)
	Info() (interface{}, error)
}

type ClusterOperator struct {
	client *Client
}

func (c *ClusterOperator) List(prefix string) ([]Operator, error) {
	panic("implement me")
}

func (c *ClusterOperator) Get(key string) (Operator, error) {
	panic("cluster operator does not support get")
}

func (c *ClusterOperator) Remove(key string) error {
	panic("cluster operator does not support remove")
}

func (c *ClusterOperator) Info() (interface{}, error) {
	leaderInfo := c.client.infoAgent.GetCurClusterInfo().LeaderInfo
	conn, err := messenger.GetRpcConnByNodeInfo(leaderInfo)
	if err != nil {
		return "", err
	}
	monitor := watcher.NewMonitorClient(conn)
	report, err := monitor.GetClusterReport(context.Background(), nil)
	return report, err
}

func (c *ClusterOperator) State() (string, error) {
	clusterInfo := c.client.infoAgent.GetCurClusterInfo()
	leaderInfo := clusterInfo.LeaderInfo
	conn, err := messenger.GetRpcConnByNodeInfo(leaderInfo)
	if err != nil {
		return "", err
	}
	monitor := watcher.NewMonitorClient(conn)
	report, err := monitor.GetClusterReport(context.Background(), &emptypb.Empty{})
	if err != nil {
		return "", err
	}
	sort.Slice(report.Nodes, func(i, j int) bool {
		return report.Nodes[i].NodeId < report.Nodes[j].NodeId
	})
	state, _ := protoToJson(report)

	clusterPipelines, err := pipeline.NewClusterPipelines(clusterInfo)
	if err != nil {
		return "", err
	}
	pipelines, _ := interfaceToJson(clusterPipelines.MetaPipelines)
	s := struct {
		Term        uint64
		LeaderID    uint64
		MetaPGNum   int32
		MetaPGSize  int32
		BlockPGNum  int32
		BlockPGSize int32
	}{
		Term:        clusterInfo.Term,
		LeaderID:    clusterInfo.LeaderInfo.RaftId,
		MetaPGNum:   clusterInfo.MetaPgNum,
		MetaPGSize:  clusterInfo.MetaPgSize,
		BlockPGNum:  clusterInfo.BlockPgNum,
		BlockPGSize: clusterInfo.BlockPgSize,
	}
	base, _ := interfaceToJson(s)

	return base + "\n" + pipelines + "\n" + state, nil
}

type VolumeOperator struct {
	volumeID string
	client   *Client
}

// Remove Deprecated
func (v *VolumeOperator) Remove(_ string) error {
	return errors.New("remove is deprecated, use DeleteBucket instead")
}

func (v *VolumeOperator) State() (string, error) {
	panic("volume operator does not support state")
}

func (v *VolumeOperator) Info() (interface{}, error) {
	panic("volume operator does not support info")
}

func (v *VolumeOperator) Get(key string) (Operator, error) {
	nodeInfo := v.client.infoAgent.GetCurClusterInfo().NodesInfo[0]
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return nil, err
	}
	moonClient := moon.NewMoonClient(conn)
	info, err := moonClient.GetInfo(context.Background(), &moon.GetInfoRequest{
		InfoType: infos.InfoType_BUCKET_INFO,
		InfoId:   infos.GenBucketID(v.volumeID, key),
	})
	if err != nil {
		return nil, err
	}
	return &BucketOperator{
		bucketInfo: info.BaseInfo.GetBucketInfo(),
		client:     v.client,
	}, err
}

func (v *VolumeOperator) List(key string) ([]Operator, error) {
	nodeInfo := v.client.infoAgent.GetCurClusterInfo().NodesInfo[0]
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return nil, err
	}
	moonClient := moon.NewMoonClient(conn)
	reply, err := moonClient.ListInfo(context.Background(), &moon.ListInfoRequest{
		InfoType: infos.InfoType_BUCKET_INFO,
		Prefix:   infos.GenBucketID(v.volumeID, key),
	})
	if err != nil {
		return nil, err
	}
	var buckets []Operator
	for _, info := range reply.BaseInfos {
		buckets = append(buckets, &BucketOperator{
			bucketInfo: info.GetBucketInfo(),
			client:     v.client,
		})
	}
	return buckets, nil
}

func (v *VolumeOperator) CreateBucket(bucketInfo *infos.BucketInfo) error {
	nodeInfo := v.client.infoAgent.GetCurClusterInfo().NodesInfo[0]
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

// DeleteBucket deletes a bucket by bucketInfo from its volume.
//
// The Bucket must be empty.
func (v *VolumeOperator) DeleteBucket(bucketInfo *infos.BucketInfo) error {
	nodeInfo := v.client.infoAgent.GetCurClusterInfo().NodesInfo[0]
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return err
	}
	moonClient := moon.NewMoonClient(conn)
	_, err = moonClient.ProposeInfo(context.Background(), &moon.ProposeInfoRequest{
		Operate:  moon.ProposeInfoRequest_DELETE,
		Id:       bucketInfo.GetID(),
		BaseInfo: bucketInfo.BaseInfo(),
	})
	return err
}

type BucketOperator struct {
	bucketInfo *infos.BucketInfo
	client     *Client
}

// List
// Deprecated
// Use Client.ListObjects instead
func (b *BucketOperator) List(prefix string) ([]Operator, error) {
	//TODO implement me
	panic("implement me")
}

func (b *BucketOperator) getAlayaClient(key string) (alaya.AlayaClient, error) {
	pgID := object.GenObjPgID(b.bucketInfo, key, b.client.infoAgent.GetCurClusterInfo().MetaPgNum)
	cp, err := pipeline.NewClusterPipelines(b.client.infoAgent.GetCurClusterInfo())
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
retry:
	alayaClient, err := b.getAlayaClient(key)
	if err != nil {
		return err
	}
	ctx, _ := alaya.SetTermToContext(context.Background(), b.client.infoAgent.GetCurClusterInfo().Term)
	_, err = alayaClient.DeleteMeta(ctx, &alaya.DeleteMetaRequest{
		ObjId: object.GenObjectId(b.bucketInfo, key),
	})
	if err != nil {
		if strings.Contains(err.Error(), errno.TermNotMatch.Error()) {
			logger.Warningf("term not match, retry")
			err = b.client.infoAgent.UpdateCurClusterInfo()
			if err != nil {
				return err
			}
			goto retry
		}
		logger.Warningf("delete meta: %v failed, err: %v", object.GenObjectId(b.bucketInfo, key), err)
	}
	return err
}

func (b *BucketOperator) State() (string, error) {
	return protoToJson(b.bucketInfo)
}

func (b *BucketOperator) Info() (interface{}, error) {
	return b.bucketInfo, nil
}

func (b *BucketOperator) Get(key string) (Operator, error) {
retry:
	alayaClient, err := b.getAlayaClient(key)
	if err != nil {
		return nil, err
	}
	ctx, _ := alaya.SetTermToContext(b.client.ctx, b.client.infoAgent.GetCurClusterInfo().Term)
	reply, err := alayaClient.GetObjectMeta(ctx, &alaya.MetaRequest{
		ObjId: object.GenObjectId(b.bucketInfo, key),
	})
	if err != nil {
		if strings.Contains(err.Error(), errno.TermNotMatch.Error()) {
			logger.Warningf("term not match, retry")
			err = b.client.infoAgent.UpdateCurClusterInfo()
			if err != nil {
				return nil, err
			}
			goto retry
		}
		return nil, err
	}
	return &ObjectOperator{meta: reply}, nil
}

type ObjectOperator struct {
	meta *object.ObjectMeta
}

func (o *ObjectOperator) List(prefix string) ([]Operator, error) {
	panic("implement me")
}

func (o *ObjectOperator) Get(key string) (Operator, error) {
	panic("object operator can not get")
}

func (o *ObjectOperator) Remove(key string) error {
	panic("object operator can not remove")
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

func interfaceToJson(data interface{}) (string, error) {
	// convert interface to json
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "marshal data error", err
	}
	var pretty bytes.Buffer
	err = json.Indent(&pretty, jsonData, "", "  ")
	if err != nil {
		return "indent data error", err
	}
	return pretty.String(), nil
}

func (o *ObjectOperator) Info() (interface{}, error) {
	return o.meta, nil
}
