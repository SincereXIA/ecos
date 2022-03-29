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
	"ecos/utils/logger"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
)

type EcosReader struct {
	infoAgent   *info_agent.InfoAgent
	clusterInfo *infos.ClusterInfo
	key         string

	blockPipes     []*pipeline.Pipeline
	curBlockIndex  int
	curBlockOffset int

	meta         *object.ObjectMeta
	objPipes     []*pipeline.Pipeline
	config       *config.ClientConfig
	cachedBlocks sync.Map

	totalChunk  uint64
	sizeOfChunk uint64
}

func (r *EcosReader) genPipelines() error {
	err := r.getObjMeta()
	if err != nil {
		return err
	}
	err = r.getHistoryClusterInfo()
	if err != nil {
		return err
	}
	r.blockPipes = pipeline.GenPipelines(*r.clusterInfo, blockPgNum, groupNum)
	return nil
}

func (r *EcosReader) getBlock(blockInfo *object.BlockInfo) ([]byte, error) {
	blockID := blockInfo.BlockId
	if block, ok := r.cachedBlocks.Load(blockID); ok {
		return block.([]byte), nil
	}
	pgID := GenBlockPG(blockInfo)
	gaiaServerId := r.blockPipes[pgID].RaftId[0]
	info, err := r.infoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(gaiaServerId, 10))
	gaiaServerInfo := info.BaseInfo().GetNodeInfo()
	if err != nil {
		logger.Errorf("get gaia server info failed, err: %v", err)
	}
	logger.Debugf("get block %10.10s from %v", blockID, gaiaServerInfo.RaftId)
	client, err := NewGaiaClient(gaiaServerInfo)
	if err != nil {
		return nil, err
	}
	req := &gaia.GetBlockRequest{
		BlockId:  blockInfo.BlockId,
		CurChunk: 0,
		Term:     r.meta.Term,
	}
	res, err := client.client.GetBlockData(context.Background(), req)
	if err != nil {
		logger.Warningf("blockClient responds err: %v", err)
		return nil, err
	}
	block := make([]byte, 0, r.config.Object.BlockSize)
	for {
		rs, err := res.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			terr := res.CloseSend()
			if terr != nil {
				logger.Infof("close gaia server failed, err: %v", terr)
			}
			logger.Warningf("res.Recv err: %v", err)
			return nil, err
		}
		block = append(block, rs.GetChunk().Content...)
	}
	r.cachedBlocks.Store(blockID, block)
	return block, nil
}

func (r *EcosReader) Read(p []byte) (n int, err error) {
	if r.meta == nil || r.objPipes == nil {
		err = r.genPipelines()
		if err != nil {
			logger.Errorf("gen pipelines failed, err: %v", err)
		}
	}
	count, pending := int64(0), len(p)

	// 预计本次需要读取几个 block
	blockNum := (pending + r.curBlockOffset) / int(r.config.Object.BlockSize)
	if blockNum*int(r.config.Object.BlockSize) < pending {
		blockNum++ // 如果最后一个 block 仍未填满，需要读取下一个 block
	}
	waitGroup := sync.WaitGroup{}

	isEOF := true              // 是否每一个 block 都已经读取完毕
	offset := r.curBlockOffset // 第一个 block 需要加入上次读的偏移
	for i := 0; i < blockNum && r.curBlockIndex < len(r.meta.Blocks); i++ {
		start := i * int(r.config.Object.BlockSize)
		end := start + int(r.config.Object.BlockSize)
		if end > len(p) {
			end = len(p)
		}
		blockInfo := r.meta.Blocks[r.curBlockIndex+i]
		waitGroup.Add(1)
		go func(info *object.BlockInfo, start int, end int, offset int) {
			block, err := r.getBlock(info)
			if err != nil {
				logger.Errorf("get block failed, err: %v", err)
				return
			}
			copy(p[start:end], block[offset:])
			atomic.AddInt64(&count, int64(minSize(end-start, len(block)-offset)))
			if end-start+offset < len(block) {
				r.curBlockOffset = end - start + offset
				logger.Debugf("CurBlockOffset: %d", r.curBlockOffset)
				isEOF = false
			} else {
				r.cachedBlocks.Delete(info.BlockId) // 已经读完，释放内存
				if offset > 0 {                     // 当前 block 不是从头读的，但已经读完，需要重置 offset，否则下一个 block 不会从头读
					r.curBlockOffset = 0
				}
			}
			waitGroup.Done()
		}(blockInfo, start, end, offset)
		offset = 0 // 后续 block 都是从头开始读
	}
	waitGroup.Wait()
	logger.Debugf("read %d bytes done", count)

	if isEOF {
		r.curBlockIndex += blockNum
	} else {
		r.curBlockIndex += blockNum - 1
	}

	if r.curBlockIndex > len(r.meta.Blocks) || (r.curBlockIndex == len(r.meta.Blocks) && isEOF) {
		r.curBlockIndex = len(r.meta.Blocks) + 1
		return int(count), io.EOF
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

func (r *EcosReader) getHistoryClusterInfo() error {
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
