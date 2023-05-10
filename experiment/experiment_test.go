package experiment

import (
	"ecos/edge-node/pipeline"
	"encoding/csv"
	"math"
	"os"
	"strconv"
	"testing"
)

func blanceTest(t *testing.T, name string, behave int) {
	// 测试 pg 数量引起的均衡性变化
	var pgNums []int
	var variances []float64
	var totalWrites []uint64

	for pgNum := 50; pgNum < 10000; pgNum += 20 {
		tester := tester{}
		totalWrite, variance, sumVariance := tester.balanceTest(pgNum, 4*1000*1000, behave, 0.5)
		pipeline.CleanCache()
		t.Logf("pgNum: %d, totalWrite: %d, variance: %f, sumVariance: %f", pgNum, totalWrite, variance, sumVariance)
		pgNums = append(pgNums, pgNum)
		variances = append(variances, sumVariance)
		totalWrites = append(totalWrites, totalWrite)
	}

	for i, pgNum := range pgNums {
		t.Logf("pgNum: %d, variance: %f", pgNum, variances[i])
	}

	data := [][]string{
		{"pgNum", "variance", "totalWrite"},
	}
	for i, pgNum := range pgNums {
		data = append(data, []string{strconv.Itoa(pgNum), strconv.FormatFloat(variances[i], 'f', -1, 64), strconv.Itoa(int(totalWrites[i]))})
	}
	filename := "pg_num_balance_" + name + ".csv"
	f, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.WriteAll(data)
	w.Flush()
}

func TestCompare(t *testing.T) {
	behave := []int{
		CEPH_LIKE,
		ECOS,
	}
	pgNum := 100
	for _, b := range behave {
		tester := tester{}
		totalWrite, variance, sumVariance := tester.balanceTest(pgNum, 4*1000*1000, b, 0.5)
		pipeline.CleanCache()
		t.Logf("pgNum: %d, totalWrite: %d, variance: %f, sumVariance: %f", pgNum, totalWrite, variance, sumVariance)
	}
}

func TestExpBalance(t *testing.T) {
	behave := []int{
		CEPH_LIKE,
		ECOS,
	}

	for _, b := range behave {
		name := "default"
		if b == CEPH_LIKE {
			name = "ceph-like"
		} else {
			name = "ecos"
		}
		blanceTest(t, name, b)
	}

}

/*
*
测试容量异构程度引起的均衡性变化
*/
func TestCapPerBlance(t *testing.T) {
	var capPers []float64
	for capPer := 0.1; capPer < 1; capPer += 0.05 {
		capPers = append(capPers, capPer)
	}
	pgNum := 1000

	behaves := []int{
		CEPH_LIKE,
		ECOS,
	}

	for _, behave := range behaves {
		var caps []float64
		var variances []float64
		var maxVariance []float64
		var totalWrites []uint64
		var stds []float64
		data := [][]string{
			{"capPer", "variance", "maxVariance", "totalWrite", "std"},
		}
		filename := "cap_per_balance_" + strconv.Itoa(behave) + ".csv"
		f, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		w := csv.NewWriter(f)
		w.WriteAll(data)
		w.Flush()

		for _, capPer := range capPers {
			tester := tester{}
			totalWrite, variance, sumVariance := tester.balanceTest(pgNum, 4*1000*1000, behave, capPer)
			pipeline.CleanCache()
			t.Logf("capPer: %f,  pgNum: %d, totalWrite: %d, variance: %f, sumVariance: %f", capPer, pgNum, totalWrite, variance, sumVariance)
			caps = append(caps, capPer)
			variances = append(variances, variance)
			maxVariance = append(maxVariance, sumVariance)
			totalWrites = append(totalWrites, totalWrite)

			small_node_capacity := uint64((1 * 1000 * 1000 * 1000 * 1000) * 1.0 * capPer)

			capacities := []uint64{
				1 * 1000 * 1000 * 1000 * 1000, // 1 TB
				1 * 1000 * 1000 * 1000 * 1000, // 1 TB
				1 * 1000 * 1000 * 1000 * 1000, // 1 TB
				1 * 1000 * 1000 * 1000 * 1000, // 1 TB
				1 * 1000 * 1000 * 1000 * 1000, // 1 TB
				small_node_capacity,
				small_node_capacity,
				small_node_capacity,
				small_node_capacity,
				small_node_capacity,
			}
			// 计算方差
			var sum float64
			for _, cap := range capacities {
				sum += float64(cap)
			}
			mean := sum / float64(len(capacities))
			var sumVar float64
			for _, cap := range capacities {
				sumVar += (float64(cap) - mean) * (float64(cap) - mean)
			}
			sumVar = sumVar / float64(len(capacities))
			std := math.Sqrt(sumVar)
			stds = append(stds, std)
			newData := []string{strconv.FormatFloat(capPer, 'f', -1, 64), strconv.FormatFloat(variance, 'f', -1, 64), strconv.FormatFloat(sumVariance, 'f', -1, 64), strconv.Itoa(int(totalWrite)), strconv.FormatFloat(std, 'f', -1, 64)}
			w.Write(newData)
			w.Flush()
		}

		//for i, capPer := range caps {
		//	data = append(data, []string{strconv.FormatFloat(capPer, 'f', -1, 64), strconv.FormatFloat(variances[i], 'f', -1, 64), strconv.FormatFloat(maxVariance[i], 'f', -1, 64), strconv.Itoa(int(totalWrites[i])), strconv.FormatFloat(stds[i], 'f', -1, 64)})
		//}

		//w.WriteAll(data)
		//w.Flush()
		f.Close()
	}
}

func TestCapAndPgNumBalance(t *testing.T) {
	var capPers []float64
	for capPer := 0.1; capPer < 1; capPer += 0.05 {
		capPers = append(capPers, capPer)
	}
	var pgNums []int
	for pgNum := 10; pgNum < 5000; pgNum += 20 {
		pgNums = append(pgNums, pgNum)
	}

	behaves := []int{
		CEPH_LIKE,
		ECOS,
	}

	for _, behave := range behaves {
		var caps []float64
		var resPgNums []int
		var variances []float64
		var totalWrites []uint64

		for _, capPer := range capPers {
			for _, pgNum := range pgNums {
				tester := tester{}
				totalWrite, variance, sumVariance := tester.balanceTest(pgNum, 4*1000*1000, behave, capPer)
				pipeline.CleanCache()
				t.Logf("capPer: %f,  pgNum: %d, totalWrite: %d, variance: %f, sumVariance: %f", capPer, pgNum, totalWrite, variance, sumVariance)
				caps = append(caps, capPer)
				resPgNums = append(resPgNums, pgNum)
				variances = append(variances, sumVariance)
				totalWrites = append(totalWrites, totalWrite)
			}
		}

		data := [][]string{
			{"capPer", "pgNum", "variance", "totalWrite"},
		}

		for i, capPer := range caps {
			data = append(data, []string{strconv.FormatFloat(capPer, 'f', -1, 64), strconv.Itoa(resPgNums[i]), strconv.FormatFloat(variances[i], 'f', -1, 64), strconv.Itoa(int(totalWrites[i]))})
		}
		filename := "cap_pg_balance_" + strconv.Itoa(behave) + ".csv"
		f, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}

		w := csv.NewWriter(f)
		w.WriteAll(data)
		w.Flush()
		f.Close()
	}
}
