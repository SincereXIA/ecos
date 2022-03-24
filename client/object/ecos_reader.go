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

	meta     *object.ObjectMeta
	objPipes []*pipeline.Pipeline

	curChunkIdInBlock uint64
	maxChunkIdInBlock uint64
	totalChunk        uint64
	curChunk          uint64
	sizeOfChunk       uint64
	chunkOffset       uint64
	alreadyReadBytes  uint64
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

func (r *EcosReader) Read(p *[]byte) (n int, err error) {
	err = r.getObjMeta()
	if err != nil {
		return 0, err
	}
	err = r.getHistoryGroupInfo()
	if err != nil {
		return 0, nil
	}
	r.blockPipes = pipeline.GenPipelines(r.groupInfo, blockPgNum, groupNum)

	count, pending := 0, cap(*p)
	for blockId, blockInfo := range r.meta.Blocks {
		if pending <= 0 {
			logger.Infof("pending == 0, break")
			break
		}
		// skip blocks already read
		if uint64(blockId) < r.curBlockId {
			continue
		}

		pgID := GenBlockPG(blockInfo)

		gaiaServerId := r.blockPipes[pgID].RaftId[0]
		gaiaServerInfo := clientNode.InfoStorage.GetNodeInfo(r.meta.Term, gaiaServerId)

		blockClient, err := NewBlockClient(gaiaServerInfo)

		if err != nil {
			return 0, nil
		}

		req := &gaia.GetBlockRequest{
			BlockId:  blockInfo.BlockId,
			CurChunk: r.curChunkIdInBlock,
			Term:     r.meta.Term,
		}

		res, err := blockClient.client.GetBlockData(context.Background(), req) // need to modify
		if err != nil {
			return 0, nil
		}

		for {
			remoteChunk, err := res.Recv()
			if err != nil {
				break
			}
			chunk := remoteChunk.GetChunk().Content

			if pending >= len(chunk) {
				*p = append(*p, chunk...)
				pending -= len(chunk)
				count += len(chunk)
				r.alreadyReadBytes += uint64(len(chunk))
				r.chunkOffset += uint64(len(chunk))
			} else {
				*p = append(*p, chunk[0:pending]...)
				count += pending
				r.alreadyReadBytes += uint64(pending)
				r.chunkOffset += uint64(pending)
			}
			if r.alreadyReadBytes == r.meta.ObjSize {
				return count, io.EOF
			}
			if r.chunkOffset >= r.sizeOfChunk {
				r.curChunk++
				r.curChunkIdInBlock++
				r.chunkOffset -= r.sizeOfChunk
				if r.curChunkIdInBlock > r.maxChunkIdInBlock {
					r.curBlockId++
					r.curChunkIdInBlock = 0
					break
				}
			}
		}
	}
	return count, nil
}

func (r *EcosReader) getObjMeta() error {
	// use key to get pdId
	pgId := GenObjectPG(r.key)

	metaServerId := r.objPipes[pgId-1].RaftId[0]
	metaServerInfo := clientNode.InfoStorage.GetNodeInfo(0, metaServerId)
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
	logger.Infof("get objMeta from raft [%v], succees, meta: %v", metaServerId, r.meta)
	// update information of EcosReader after get newer Meta
	r.updateEcosReader()
	return nil
}

func (r *EcosReader) getHistoryGroupInfo() error {
	r.groupInfo = clientNode.InfoStorage.GetGroupInfo(r.meta.Term)
	// r.groupInfo = clientNode.InfoStorage.GetGroupInfo(0)
	return nil
}

func (r *EcosReader) updateEcosReader() {
	r.totalChunk = r.meta.ObjSize / r.sizeOfChunk
	if r.meta.ObjSize > r.totalChunk*r.sizeOfChunk {
		r.totalChunk++
	}
}
