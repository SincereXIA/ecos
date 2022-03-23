package object

import (
	"context"
	clientNode "ecos/client/node"
	"ecos/edge-node/gaia"
	"ecos/edge-node/node"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/logger"
	"io"
)

type EcosReader struct {
	groupInfo *node.GroupInfo
	key       string

	blockPipes []*pipeline.Pipeline
	curBlockId uint64
	maxBlockId uint64

	meta     *object.ObjectMeta
	objPipes []*pipeline.Pipeline

	curChunkId  uint64
	maxChunkId  uint64
	sizeOfChunk uint64
	chunkOffset uint64
}

type BlockClient struct {
	serverNode *node.NodeInfo
	client     gaia.GaiaClient
	context    context.Context
	cancel     context.CancelFunc
}

func NewBlockClient(serverNode *node.NodeInfo) (*BlockClient, error) {

	blockClient := &BlockClient{
		serverNode: serverNode,
		context:    context.Background(),
		cancel:     nil,
	}

	conn, err := messenger.GetRpcConnByInfo(serverNode)

	if err != nil {
		return nil, err
	}

	blockClient.client = gaia.NewGaiaClient(conn)

	return blockClient, nil
}

func (r *EcosReader) Read(p []byte) (n int, err error) {
	// Read finished, return EOF
	if r.curBlockId == r.maxBlockId && r.curChunkId == r.maxChunkId && r.chunkOffset == r.sizeOfChunk {
		return 0, io.EOF
	}

	err = r.getObjMeta()
	if err != nil {
		return 0, err
	}
	err = r.getHistoryGroupInfo()
	if err != nil {
		return 0, nil
	}
	r.blockPipes = pipeline.GenPipelines(r.groupInfo, blockPgNum, groupNum)

	count, pending := 0, len(p)
	for blockId, blockInfo := range r.meta.Blocks {
		// skip blocks already read
		if uint64(blockId) < r.curBlockId {
			continue
		}

		pgID := GenBlockPG(blockInfo)

		gaiaServerId := r.blockPipes[pgID].RaftId[0]
		gaiaServerInfo := r.groupInfo.NodesInfo[gaiaServerId]

		blockClient, err := NewBlockClient(gaiaServerInfo)

		if err != nil {
			return 0, nil
		}

		req := &gaia.GetBlockRequest{
			BlockId:  blockInfo.BlockId,
			CurChunk: r.curChunkId,
			Term:     r.meta.Term,
		}

		res, err := blockClient.client.GetBlockData(context.Background(), req) // need to modify
		if err != nil {
			return 0, nil
		}

		for {
			if pending <= 0 {
				logger.Infof("pending == 0, break")
				return count, nil
			}
			remoteChunk, err := res.Recv()
			chunk := remoteChunk.GetChunk().Content
			if err != nil {
				logger.Errorf("get Chunk failed, err: %v", err)
				return 0, nil
			}
			if pending >= len(chunk) {
				p = append(p, chunk...)
				pending -= len(chunk)
				count += len(chunk)
				r.chunkOffset += uint64(len(chunk))
			} else {
				p = append(p, chunk[0:pending]...)
				count += pending
				r.chunkOffset += uint64(pending)
			}
			if r.chunkOffset >= r.sizeOfChunk {
				r.curChunkId++
				r.chunkOffset -= r.sizeOfChunk
				if r.curChunkId >= r.maxChunkId {
					r.curBlockId++
					r.curChunkId -= r.maxChunkId
				}
			}
		}
	}
	return count, nil
}

func (r *EcosReader) getObjMeta() error {
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
	// update information of EcosReader after get newer Meta
	r.updateEcosReader()
	return nil
}

func (r *EcosReader) getHistoryGroupInfo() error {
	// get group info
	c, err := clientNode.NewClientNodeInfoStorage()
	if err != nil {
		logger.Errorf("failed to create client node info storage")
		return err
	}
	r.groupInfo = c.GetGroupInfo(r.meta.Term)
	return nil
}

func (r *EcosReader) updateEcosReader() {
	r.maxBlockId = uint64(len(r.meta.Blocks) - 1)
}
