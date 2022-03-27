package object

import (
	"context"
	"ecos/client/config"
	info_agent "ecos/client/info-agent"
	clientNode "ecos/client/node"
	"ecos/edge-node/gaia"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/messenger"
	"ecos/utils/logger"
	"io"
	"strconv"
)

type EcosReader struct {
	infoAgent   *info_agent.InfoAgent
	clusterInfo *infos.ClusterInfo
	key         string

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
	serverNode *infos.NodeInfo
	client     gaia.GaiaClient
	context    context.Context
	cancel     context.CancelFunc
}

func NewBlockClient(serverNode *infos.NodeInfo) (*BlockClient, error) {

	blockClient := &BlockClient{
		serverNode: serverNode,
	}

	conn, err := messenger.GetRpcConnByNodeInfo(serverNode)
	if err != nil {
		return nil, err
	}
	blockClient.client = gaia.NewGaiaClient(conn)

	if configTimeout := config.Config.UploadTimeout; configTimeout > 0 {
		blockClient.context, blockClient.cancel = context.WithTimeout(context.Background(), configTimeout)
	} else {
		blockClient.context = context.Background()
		blockClient.cancel = nil
	}

	return blockClient, nil
}

func (r *EcosReader) Read(p []byte) (n int, err error) {
	err = r.getObjMeta()
	if err != nil {
		logger.Warningf("get obj meta failed, err: %v", err)
		return 0, err
	}
	err = r.getHistoryGroupInfo()
	if err != nil {
		return 0, nil
	}
	r.blockPipes = pipeline.GenPipelines(*r.clusterInfo, blockPgNum, groupNum)

	count, pending, curPidx := uint64(0), uint64(len(p)), 0
	for blockId, blockInfo := range r.meta.Blocks {
		logger.Infof("current block id [%v]", blockId)
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
		logger.Infof("gaia server id is %v", gaiaServerId)
		info, err := r.infoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(gaiaServerId, 10))
		gaiaServerInfo := info.BaseInfo().GetNodeInfo()
		if err != nil {
			logger.Errorf("get gaia server info failed, err: %v", err)
		}
		blockClient, err := NewBlockClient(gaiaServerInfo)
		if err != nil {
			return 0, err
		}
		req := &gaia.GetBlockRequest{
			BlockId:  blockInfo.BlockId,
			CurChunk: r.curChunkIdInBlock,
			Term:     r.meta.Term,
		}
		res, err := blockClient.client.GetBlockData(context.Background(), req)
		if err != nil {
			logger.Warningf("blockClient responds err: %v", err)
			return 0, err
		}

		for {
			remoteChunk, err := res.Recv()
			logger.Warningf("res.Recv err: %v", err)
			if err == io.EOF {
				logger.Infof("next block")
				break
			}
			if err != nil {
				terr := res.CloseSend()
				if terr != nil {
					logger.Infof("close gaia server failed, err: %v", terr)
				}
				logger.Warningf("res.Recv err: %v", err)
				return int(count), err
			}
			chunk := remoteChunk.GetChunk().Content
			readBytes := remoteChunk.GetChunk().ReadBytes
			if pending >= readBytes {
				copy(p[curPidx:], chunk[r.chunkOffset:])
				pending -= readBytes
				count += readBytes
				curPidx += int(readBytes)
				r.alreadyReadBytes += readBytes
				r.chunkOffset += readBytes
			} else {
				copy(p[curPidx:], chunk[0:pending])
				count += pending
				curPidx += int(pending)
				r.alreadyReadBytes += pending
				r.chunkOffset += pending
			}
			if r.alreadyReadBytes == r.meta.ObjSize {
				terr := res.CloseSend()
				if terr != nil {
					logger.Infof("close gaia server failed, err: %v", terr)
				}
				return int(count), io.EOF
			}
			if r.chunkOffset >= r.sizeOfChunk {
				r.curChunk++
				r.curChunkIdInBlock++
				r.chunkOffset = 0
			}
			if r.curChunkIdInBlock > r.maxChunkIdInBlock {
				r.curBlockId++
				r.curChunkIdInBlock = 0
				break
			}
		}
	}
	return int(count), nil
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
	info, err := r.infoAgent.Get(infos.InfoType_CLUSTER_INFO, strconv.FormatUint(r.meta.Term, 10))
	if err != nil {
		logger.Errorf("get term [%v] cluster info failed, err: %v", err)
		return err
	}
	r.clusterInfo = info.BaseInfo().GetClusterInfo()
	return nil
}

func (r *EcosReader) updateEcosReader() {
	r.totalChunk = r.meta.ObjSize / r.sizeOfChunk
	if r.meta.ObjSize > r.totalChunk*r.sizeOfChunk {
		r.totalChunk++
	}
}
