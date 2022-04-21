package io

import (
	"context"
	"ecos/client/config"
	agent "ecos/client/info-agent"
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
	infoAgent   *agent.InfoAgent
	clusterInfo *infos.ClusterInfo
	bucketInfo  *infos.BucketInfo
	key         string

	blockPipes     []*pipeline.Pipeline
	curBlockIndex  int
	curBlockOffset int

	meta         *object.ObjectMeta
	objPipes     []*pipeline.Pipeline
	config       *config.ClientConfig
	cachedBlocks sync.Map
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
	pgID := object.GenBlockPgID(blockInfo.BlockId, r.clusterInfo.BlockPgNum)
	gaiaServerId := r.blockPipes[pgID-1].RaftId[0]
	info, err := r.infoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(gaiaServerId, 10))
	gaiaServerInfo := info.BaseInfo().GetNodeInfo()
	if err != nil {
		logger.Errorf("get gaia server info failed, err: %v", err)
	}
	logger.Debugf("get block %10.10s from %v", blockID, gaiaServerInfo.RaftId)
	client, err := NewGaiaClient(gaiaServerInfo, r.config)
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
	block := make([]byte, 0, r.bucketInfo.Config.BlockSize)
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
	calcBlocks, err := r.calcBlocks(pending)
	waitGroup := sync.WaitGroup{}

	for _, block := range calcBlocks {
		blockInfo := r.meta.Blocks[block.blockIndex]
		waitGroup.Add(1)
		go func(info *object.BlockInfo, bufferL, bufferR, blockL, blockR int) {
			block, err := r.getBlock(info)
			if err != nil {
				logger.Errorf("get block failed, err: %v", err)
				return
			}
			copy(p[bufferL:bufferR], block[blockL:blockR])
			atomic.AddInt64(&count, int64(bufferR-bufferL))
			waitGroup.Done()
		}(blockInfo, block.bufferStart, block.bufferEnd, block.blockStart, block.blockEnd)
	}
	waitGroup.Wait()
	logger.Debugf("read %d bytes done", count)

	return int(count), err
}

func (r *EcosReader) getObjMeta() error {
	// use key to get pdId
	pgId := object.GenObjPgID(r.bucketInfo, r.key, 10)
	metaServerIdString := strconv.FormatUint(r.objPipes[pgId-1].RaftId[0], 10)
	metaServerInfo, _ := r.infoAgent.Get(infos.InfoType_NODE_INFO, metaServerIdString)
	metaClient, err := NewMetaClient(metaServerInfo.BaseInfo().GetNodeInfo(), r.config)
	if err != nil {
		logger.Errorf("New meta client failed, err: %v", err)
		return err
	}

	objID := object.GenObjectId(r.bucketInfo, r.key)
	r.meta, err = metaClient.GetObjMeta(objID)
	if err != nil {
		logger.Errorf("get objMeta failed, err: %v", err)
		return err
	}
	logger.Infof("get objMeta from raft [%v], success, meta: %v", metaServerIdString, r.meta)
	return nil
}

func (r *EcosReader) getHistoryClusterInfo() error {
	info, err := r.infoAgent.Get(infos.InfoType_CLUSTER_INFO, strconv.FormatUint(r.meta.Term, 10))
	if err != nil {
		logger.Errorf("get term %v cluster info failed, err: %v", r.meta.Term, err)
		return err
	}
	r.clusterInfo = info.BaseInfo().GetClusterInfo()
	return nil
}

type calcBlock struct {
	blockIndex  int
	blockStart  int
	blockEnd    int
	bufferStart int
	bufferEnd   int
}

// calcBlocks 计算需要读取的 block 列表
func (r *EcosReader) calcBlocks(expect int) ([]calcBlock, error) {
	// 计算需要读取的 block 列表
	var blocks []calcBlock
	blockOffset := r.curBlockOffset
	bufferOffset := 0
	pending := expect
	finish := false
	for i := r.curBlockIndex; i < len(r.meta.Blocks) && !finish; i++ {
		block := r.meta.Blocks[i]
		if uint64(blockOffset+pending) >= block.BlockSize {
			blocks = append(blocks, calcBlock{
				blockIndex:  i,
				blockStart:  blockOffset,
				blockEnd:    int(block.BlockSize),
				bufferStart: bufferOffset,
				bufferEnd:   bufferOffset + int(block.BlockSize) - blockOffset,
			})
			bufferOffset += int(block.BlockSize) - blockOffset
			blockOffset = 0
		} else {
			blocks = append(blocks, calcBlock{
				blockIndex:  i,
				blockStart:  blockOffset,
				blockEnd:    blockOffset + pending,
				bufferStart: bufferOffset,
				bufferEnd:   bufferOffset + pending,
			})
			bufferOffset += pending
			blockOffset += pending
			finish = true
		}
		r.curBlockIndex = i
		r.curBlockOffset = blockOffset
		pending = expect - bufferOffset
	}
	if !finish {
		return blocks, io.EOF
	}
	return blocks, nil
}
