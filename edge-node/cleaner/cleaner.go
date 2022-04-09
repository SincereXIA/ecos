package cleaner

import (
	"context"
	"ecos/edge-node/gaia"
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/edge-node/watcher"
	"ecos/messenger"
	"strconv"
)

type Cleaner struct {
	ctx     context.Context
	watcher *watcher.Watcher
}

func (c *Cleaner) removeBlock(term uint64, blockInfo *object.BlockInfo) error {
	clusterInfo := c.watcher.GetCurrentClusterInfo()
	pgID := object.GenBlockPgID(blockInfo.BlockId, clusterInfo.BlockPgNum)
	clusterPipelines, _ := pipeline.NewClusterPipelines(&clusterInfo)
	gaiaID := clusterPipelines.GetBlockPG(pgID)[0]
	info, err := c.watcher.GetMoon().GetInfoDirect(infos.InfoType_NODE_INFO, strconv.FormatUint(gaiaID, 10))
	if err != nil {
		return err
	}
	nodeInfo := info.BaseInfo().GetNodeInfo()
	conn, err := messenger.GetRpcConnByNodeInfo(nodeInfo)
	if err != nil {
		return err
	}
	gaiaClient := gaia.NewGaiaClient(conn)
	_, err = gaiaClient.DeleteBlock(c.ctx, &gaia.DeleteBlockRequest{
		BlockId:  blockInfo.BlockId,
		Pipeline: clusterPipelines.GetBlockPipeline(pgID),
		Term:     term,
	})
	return err
}

func (c *Cleaner) RemoveObjectBlocks(meta *object.ObjectMeta) error {
	for _, blockInfo := range meta.Blocks {
		err := c.removeBlock(meta.Term, blockInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewCleaner(ctx context.Context, watcher *watcher.Watcher) *Cleaner {
	return &Cleaner{
		ctx:     ctx,
		watcher: watcher,
	}
}
