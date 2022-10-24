package experiment

import (
	"ecos/edge-node/infos"
	"ecos/edge-node/object"
	"ecos/edge-node/pipeline"
	"ecos/utils/logger"
	"errors"
	"math/rand"
	"strconv"
)

type MockCluster struct {
	clusterInfo        infos.ClusterInfo
	volumesTotal       []uint64
	volumesUsed        []uint64
	lastTermVolumeUsed []uint64
	pipelines          *pipeline.ClusterPipelines

	blockPgNum  int32
	blockPgSize int32
}

func (c *MockCluster) PutBlock(blockID string, size uint64) error {
	pgID := object.GenBlockPgID(blockID, c.clusterInfo.BlockPgNum)
	pg := c.pipelines.GetBlockPipeline(pgID)
	for i := 0; i < len(pg.RaftId); i++ {
		c.volumesUsed[pg.RaftId[i]-1] += size
		if c.volumesUsed[pg.RaftId[i]-1] > c.volumesTotal[pg.RaftId[i]-1] {
			return errors.New("no enough space")
		}
	}
	// output every 100 times
	if rand.Intn(1000) == 0 {
		difference := c.CheckDifference()
		logger.Infof("difference: %v", difference)
		if difference > 0.001 {
			c.ProposeNewClusterInfo()
		}
	}
	return nil
}

func (c *MockCluster) ProposeNewClusterInfo() {
	var nodesInfo []*infos.NodeInfo
	for i := 0; i < len(c.volumesTotal); i++ {
		nodesInfo = append(nodesInfo, &infos.NodeInfo{
			RaftId:   uint64(i + 1),
			Uuid:     "uuid-" + strconv.Itoa(i+1),
			Capacity: c.volumesTotal[i] - c.volumesUsed[i],
			State:    infos.NodeState_ONLINE,
		})
	}
	clusterInfo := infos.ClusterInfo{
		Term:            c.clusterInfo.Term + 1,
		LeaderInfo:      nodesInfo[0],
		NodesInfo:       nodesInfo,
		UpdateTimestamp: nil,
		MetaPgNum:       10,
		MetaPgSize:      3,
		BlockPgNum:      c.blockPgNum,
		BlockPgSize:     c.blockPgSize,
		LastTerm:        c.clusterInfo.Term,
	}
	c.clusterInfo = clusterInfo
	c.pipelines, _ = pipeline.NewClusterPipelines(clusterInfo)
	for i := 0; i < len(c.volumesTotal); i++ {
		c.lastTermVolumeUsed[i] = c.volumesUsed[i]
	}
	logger.Debugf("ProposeNewClusterInfo: %v", clusterInfo.Term)
}

func (c *MockCluster) PrintRemainVolumePercent() {
	logger.Infof("Term: %v", c.clusterInfo.Term)
	for i := 0; i < len(c.volumesTotal); i++ {
		remain := float64(c.volumesTotal[i]-c.volumesUsed[i]) / float64(c.volumesTotal[i])
		logger.Infof("Node %d: %v", i+1, remain)
	}
}

func NewMockCluster(capacities []uint64, blockPgNum int) *MockCluster {
	blockPgSize := 3
	var nodesInfo []*infos.NodeInfo
	for i := 0; i < len(capacities); i++ {
		nodesInfo = append(nodesInfo, &infos.NodeInfo{
			RaftId:   uint64(i + 1),
			Uuid:     "uuid-" + strconv.Itoa(i+1),
			Capacity: capacities[i],
			State:    infos.NodeState_ONLINE,
		})
	}

	clusterInfo := infos.ClusterInfo{
		Term:            1,
		LeaderInfo:      nodesInfo[0],
		NodesInfo:       nodesInfo,
		UpdateTimestamp: nil,
		MetaPgNum:       10,
		MetaPgSize:      3,
		BlockPgNum:      int32(blockPgNum),
		BlockPgSize:     int32(blockPgSize),
		LastTerm:        0,
	}
	clusterPipelines, _ := pipeline.NewClusterPipelines(clusterInfo)
	volumesUsed := make([]uint64, len(capacities))
	lastTermVolumesUsed := make([]uint64, len(capacities))
	cluster := &MockCluster{
		clusterInfo:        clusterInfo,
		volumesTotal:       capacities,
		volumesUsed:        volumesUsed,
		lastTermVolumeUsed: lastTermVolumesUsed,
		pipelines:          clusterPipelines,
		blockPgSize:        int32(blockPgSize),
		blockPgNum:         int32(blockPgNum),
	}
	p := clusterPipelines.BlockPipelines
	count := make([]int, len(nodesInfo))
	// 统计每个节点上的pg数量
	for i := 0; i < len(p); i++ {
		for j := 0; j < len(p[i].RaftId); j++ {
			count[p[i].RaftId[j]-1] += 1
		}
	}
	for i := 0; i < len(count); i++ {
		logger.Infof("Node %d: %d", i+1, count[i])
	}
	return cluster
}

func (c *MockCluster) CheckDifference() float64 {
	var diff []uint64
	for i := 0; i < len(c.volumesUsed); i++ {
		diff = append(diff, c.volumesUsed[i]-c.lastTermVolumeUsed[i])
	}
	var usePercent []float64
	for i := 0; i < len(diff); i++ {
		usePercent = append(usePercent, float64(diff[i])/float64(c.volumesTotal[i]-c.lastTermVolumeUsed[i]))
	}
	average := 0.0
	for i := 0; i < len(usePercent); i++ {
		average += usePercent[i]
	}
	average /= float64(len(usePercent))

	variance := 0.0
	for i := 0; i < len(usePercent); i++ {
		variance += (usePercent[i] - average) * (usePercent[i] - average)
	}
	variance /= float64(len(usePercent) - 1)
	return variance
}

func balanceTest() {
	capacities := []uint64{
		1 * 1000 * 1000 * 1000 * 1000, // 1 TB
		1 * 1000 * 1000 * 1000 * 1000, // 1 TB
		1 * 1000 * 1000 * 1000 * 1000, // 1 TB
		1 * 1000 * 1000 * 1000 * 1000, // 1 TB
		1 * 1000 * 1000 * 1000 * 1000, // 1 TB
		500 * 1000 * 1000 * 1000,      // 500 GB
		500 * 1000 * 1000 * 1000,      // 500 GB
		500 * 1000 * 1000 * 1000,      // 500 GB
	}
	//capacities := []int{
	//	1000, 1000, 1000, 1000, 1000, 500, 500, 500,
	//}

	cluster := NewMockCluster(capacities, 1000)

	r := rand.New(rand.NewSource(99))

	bid := 0
	for {
		// 文件大小（B），平均 16MB, 波动 8MB
		size := int64(16*1000*1000 + (r.NormFloat64() * 8 * 1000 * 1000))
		if size < 0 {
			size = 0
		}
		var blocks []uint64
		for size > 0 {
			b := int64(4 * 1000 * 1000)
			if size < b {
				b = size
			}
			blocks = append(blocks, uint64(b))
			size -= b
		}
		end := false
		for _, blockSize := range blocks {
			bid += 1
			blockID := strconv.Itoa(bid)
			err := cluster.PutBlock(blockID, blockSize)
			if err != nil {
				end = true
				break
			}
			if end {
				break
			}
		}
		if end {
			break
		}
	}
	cluster.PrintRemainVolumePercent()
}
