package io

import (
	"context"
	"ecos/client/config"
	"ecos/cloud/rainbow"
	"ecos/common/gaia"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"ecos/messenger/common"
	"ecos/utils/errno"
	"ecos/utils/logger"
	"fmt"
	"github.com/rcrowley/go-metrics"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type EcosReader struct {
	ctx context.Context
	f   *EcosIOFactory

	clusterInfo *infos.ClusterInfo
	pipes       *pipeline.ClusterPipelines

	curBlockIndex  int
	curBlockOffset int

	key          string
	meta         *object.ObjectMeta
	cachedBlocks sync.Map

	// For Go Metrics
	startTime time.Time
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
	r.pipes, err = pipeline.NewClusterPipelines(*r.clusterInfo)
	return err
}

func (r *EcosReader) GetBlock(blockInfo *object.BlockInfo) ([]byte, error) {
	blockID := blockInfo.BlockId
	if block, ok := r.cachedBlocks.Load(blockID); ok {
		return block.([]byte), nil
	}
	var block []byte
	var err error
	switch r.f.config.ConnectType {
	case config.ConnectCloud:
		block, err = r.GetBlockByCloud(blockInfo)
	default:
		block, err = r.GetBlockByEdge(blockInfo)
	}
	if err != nil {
		return nil, err
	}
	r.cachedBlocks.Store(blockID, block)
	return block, nil
}

func (r *EcosReader) GetBlockByCloud(blockInfo *object.BlockInfo) ([]byte, error) {
	conn, err := messenger.GetRpcConn(r.f.config.CloudAddr, r.f.config.CloudPort)
	if err != nil {
		return nil, err
	}
	client := gaia.NewGaiaClient(conn)
	if err != nil {
		return nil, err
	}
	req := &gaia.GetBlockRequest{
		BlockId:  blockInfo.BlockId,
		CurChunk: 0,
		Term:     r.meta.Term,
	}
	res, err := client.GetBlockData(r.ctx, req)
	defer func(res gaia.Gaia_GetBlockDataClient) {
		err := res.CloseSend()
		if err != nil {
			logger.Errorf("close stream failed, err: %v", err)
		}
	}(res)
	if err != nil {
		logger.Warningf("blockClient responds err: %v", err)
		return nil, err
	}
	block := make([]byte, 0, r.f.bucketInfo.Config.BlockSize)
	for {
		rs, err := res.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Warningf("res.Recv err: %v", err)
			return nil, err
		}
		block = append(block, rs.GetChunk().Content...)
	}
	return block, nil
}

func (r *EcosReader) GetBlockInEdgeByCloud(blockInfo *object.BlockInfo) ([]byte, error) {
	conn, err := messenger.GetRpcConn(r.f.config.CloudAddr, r.f.config.CloudPort)
	if err != nil {
		return nil, err
	}
	client := rainbow.NewRainbowClient(conn)

	stream, err := client.SendRequest(r.ctx, &rainbow.Request{
		Method:    rainbow.Request_GET,
		Resource:  rainbow.Request_BLOCK,
		RequestId: blockInfo.BlockId,
		InfoType:  0,
		Info:      nil,
		Meta:      nil,
	})
	if err != nil {
		return nil, err
	}
	block := make([]byte, 0, blockInfo.BlockSize)
	for {
		res, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if res.Result.Status != common.Result_OK {
			return nil, fmt.Errorf("get block %s failed, err: %s", blockInfo.BlockId, res.Result.Message)
		}
		block = append(block, res.Chunk...)
	}
	return block, nil
}

func (r *EcosReader) GetBlockByEdge(blockInfo *object.BlockInfo) ([]byte, error) {
	pgID := object.GenBlockPgID(blockInfo.BlockId, r.clusterInfo.BlockPgNum)
	gaiaServerId := r.pipes.GetBlockPG(pgID)
	info, err := r.f.infoAgent.Get(infos.InfoType_NODE_INFO, strconv.FormatUint(gaiaServerId[0], 10))
	gaiaServerInfo := info.BaseInfo().GetNodeInfo()
	if err != nil {
		logger.Errorf("get gaia server info failed, err: %v", err)
	}
	logger.Debugf("get block %10.10s from %v", blockInfo.BlockId, gaiaServerInfo.RaftId)
	client, err := NewGaiaClient(gaiaServerInfo, r.f.config)
	if err != nil {
		return nil, err
	}
	req := &gaia.GetBlockRequest{
		BlockId:  blockInfo.BlockId,
		CurChunk: 0,
		Term:     r.meta.Term,
	}
	res, err := client.client.GetBlockData(r.ctx, req)
	if err != nil {
		logger.Warningf("blockClient responds err: %v", err)
		return nil, err
	}
	block := make([]byte, 0, r.f.bucketInfo.Config.BlockSize)
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
	return block, nil
}

func (r *EcosReader) Read(p []byte) (n int, err error) {
	if r.meta == nil || r.pipes == nil {
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
			block, err := r.GetBlock(info)
			if err != nil {
				logger.Errorf("get block failed, err: %v", err)
				return
			}
			logger.Tracef("block %10.10s size: %d", info.BlockId, len(block))
			copy(p[bufferL:bufferR], block[blockL:blockR])
			atomic.AddInt64(&count, int64(bufferR-bufferL))
			waitGroup.Done()
			if blockR == len(block) {
				r.cachedBlocks.Delete(info.BlockId) // Finish read, delete from cache
			}
		}(blockInfo, block.bufferStart, block.bufferEnd, block.blockStart, block.blockEnd)
	}
	waitGroup.Wait()
	logger.Tracef("read %d bytes done", count)

	if err == io.EOF {
		metrics.GetOrRegisterTimer(watcher.MetricsClientGetTimer, nil).UpdateSince(r.startTime)
	}
	return int(count), err
}

func (r *EcosReader) getObjMeta() error {
retry:
	metaClient, err := NewMetaAgent(r.ctx, r.f.config, r.f.infoAgent, r.f.bucketInfo)
	if err != nil {
		logger.Errorf("New meta client failed, err: %v", err)
		return err
	}
	r.meta, err = metaClient.GetObjMeta(r.key)
	if err != nil {
		if strings.Contains(err.Error(), errno.TermNotMatch.Error()) {
			err = r.f.infoAgent.UpdateCurClusterInfo()
			if err != nil {
				return err
			}
			goto retry
		}
		logger.Errorf("get objMeta failed, err: %v", err)
		return err
	}
	logger.Infof("get objMeta success, meta: %v", r.meta)
	return nil
}

func (r *EcosReader) getHistoryClusterInfo() error {
	info, err := r.f.infoAgent.Get(infos.InfoType_CLUSTER_INFO, strconv.FormatUint(r.meta.Term, 10))
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

func (r *EcosReader) Seek(offset int64, whence int) (newOffset int64, err error) {
	errFmt := "seek out of range, offset: %d, size: %d"
	if r.meta == nil || r.pipes == nil {
		err = r.genPipelines()
		if err != nil {
			logger.Errorf("gen pipelines failed, err: %v", err)
			return 0, err
		}
	}
	var curOffset int64 = 0
	for i := 0; i <= r.curBlockIndex; i++ {
		curOffset += int64(r.meta.Blocks[i].BlockSize)
	}
	switch whence {
	case io.SeekStart:
		newOffset = offset
		r.curBlockIndex = 0
		r.curBlockOffset = 0
	case io.SeekCurrent:
		newOffset = curOffset + offset
	case io.SeekEnd:
		newOffset = int64(r.meta.ObjSize) - offset
		offset = newOffset - curOffset
	default:
		return 0, errno.IllegalStatus
	}
	_, err = r.calcBlocks(int(offset))
	if err != nil {
		return
	}
	if newOffset < 0 || newOffset > int64(r.meta.ObjSize) {
		return 0, fmt.Errorf(errFmt, newOffset, r.meta.ObjSize)
	}
	return
}
