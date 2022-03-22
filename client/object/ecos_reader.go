package object

import (
	clientNode "ecos/client/node"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/utils/logger"
)

type EcosReader struct {
	groupInfo *node.GroupInfo
	key       string

	meta     *object.ObjectMeta
	objPipes []*pipeline.Pipeline
}

func (r *EcosReader) Read(p []byte) (n int, err error) {
	// TODO:
	return 0, nil
}

func (r *EcosReader) getObjMeta() error {
	// get group info
	c, err := clientNode.NewClientNodeInfoStorage()
	if err != nil {
		logger.Errorf("failed to create client node info storage")
	}
	r.groupInfo = c.GetGroupInfo(0)

	// use key to get pdId
	pdId := GenObjectPG(r.key)

	metaServerId := r.objPipes[pdId-1].RaftId[0]
	metaServerInfo := r.groupInfo.NodesInfo[metaServerId]

	metaClient, err := NewMetaClient(metaServerInfo)
	if err != nil {
		logger.Errorf("New meta client failed, err: %v", err)
		return err
	}

	objID := GenObjectId(r.key)

	r.meta, err = metaClient.GetObjMeta(objID)

	if err != nil {
		logger.Errorf("get objMeta failed, err: %v", err)
		return err
	}
	return nil
}
