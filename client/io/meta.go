package io

import (
	"context"
	"ecos/client/config"
	info_agent "ecos/client/info-agent"
	"ecos/cloud/rainbow"
	"ecos/edge-node/alaya"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/messenger/common"
	"errors"
)

type MetaAgent struct {
	ctx  context.Context
	conf *config.ClientConfig

	infoAgent  *info_agent.InfoAgent
	bucketInfo *infos.BucketInfo
}

// NewMetaAgent create a client with config
func NewMetaAgent(ctx context.Context, conf *config.ClientConfig, infoAgent *info_agent.InfoAgent,
	bucketInfo *infos.BucketInfo) (*MetaAgent, error) {
	var newClient = MetaAgent{
		ctx:        ctx,
		conf:       conf,
		infoAgent:  infoAgent,
		bucketInfo: bucketInfo,
	}
	return &newClient, nil
}

// SubmitMeta provides a way to upload ObjectMeta to AlayaServer
func (c *MetaAgent) SubmitMeta(meta *object.ObjectMeta) (*common.Result, error) {
	objID := meta.ObjId
	_, _, key, _, _ := object.SplitID(objID)

	client, err := c.getAlayaClient(key)
	if err != nil {
		return nil, err
	}
	ctx, _ := alaya.SetTermToContext(c.ctx, c.infoAgent.GetCurClusterInfo().Term)

	return client.RecordObjectMeta(ctx, meta)
}

// GetObjMeta provides a way to get ObjectMeta from AlayaServer
func (c *MetaAgent) GetObjMeta(key string) (*object.ObjectMeta, error) {
	switch c.conf.ConnectType {
	case config.ConnectCloud:
		return c.GetObjMetaByCloud(key)
	case config.ConnectEdge:
		return c.GetObjMetaByEdge(key)
	default:
		return nil, errors.New("unknown connect type")
	}
}

func (c *MetaAgent) getAlayaClient(key string) (alaya.AlayaClient, error) {
	clusterInfo := c.infoAgent.GetCurClusterInfo()
	pgId := object.GenObjPgID(c.bucketInfo, key, clusterInfo.MetaPgNum)
	pipes, _ := pipeline.NewClusterPipelines(clusterInfo)

	metaServerIdString := pipes.GetBlockPGNodeID(pgId)[0]
	info, _ := c.infoAgent.Get(infos.InfoType_NODE_INFO, metaServerIdString)
	nodeInfo := info.BaseInfo().GetNodeInfo()

	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return nil, err
	}
	client := alaya.NewAlayaClient(conn)
	return client, nil
}

func (c *MetaAgent) GetObjMetaByEdge(key string) (*object.ObjectMeta, error) {
	client, err := c.getAlayaClient(key)
	if err != nil {
		return nil, err
	}
	ctx, _ := alaya.SetTermToContext(c.ctx, c.infoAgent.GetCurClusterInfo().Term)

	objID := object.GenObjectId(c.bucketInfo, key)

	metaReq := alaya.MetaRequest{
		ObjId: objID,
	}

	return client.GetObjectMeta(ctx, &metaReq)
}

func (c *MetaAgent) GetObjMetaByCloud(key string) (*object.ObjectMeta, error) {
	conn, err := messenger.GetRpcConn(c.conf.CloudAddr, c.conf.CloudPort)
	if err != nil {
		return nil, err
	}
	client := rainbow.NewRainbowClient(conn)

	objID := object.GenObjectId(c.bucketInfo, key)

	stream, err := client.SendRequest(c.ctx, &rainbow.Request{
		Method:    rainbow.Request_GET,
		Resource:  rainbow.Request_META,
		RequestId: objID,
	})

	if err != nil {
		return nil, err
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	return resp.Metas[0], nil
}
