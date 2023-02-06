package experiment

import (
	"ecos/edge-node/pipeline"
	"testing"
)

func TestExpBalance(t *testing.T) {
	behave := CEPH_LIKE

	// 测试 pg 数量引起的均衡性变化
	var pgNums []int
	var variances []float64
	for pgNum := 100; pgNum < 10000; pgNum += 1000 {
		totalWrite, variance := balanceTest(pgNum, 4*1000*1000, behave)
		pipeline.CleanCache()
		t.Logf("pgNum: %d, totalWrite: %d, variance: %f", pgNum, totalWrite, variance)
		pgNums = append(pgNums, pgNum)
		variances = append(variances, variance)
	}

	for i, pgNum := range pgNums {
		t.Logf("pgNum: %d, variance: %f", pgNum, variances[i])
	}
}
