package cmd

import (
	"context"
	"ecos/client/config"
	"ecos/experiment/exp-client/benchmark"
	"ecos/utils/common"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var runCmd = &cobra.Command{
	Use:   "run {put | get | benchmark}",
	Short: "run exp client",
	Long:  `Run exp client with default configuration.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		benchmark.NetWorkTimeInterval = time.Duration(timeInterval) * time.Second
		size, err := common.ParseSize(limitTestSizeStr)
		if err != nil {
			logger.Errorf("Parse limit test size fail: %v", err)
			os.Exit(1)
		}
		limitTestSize = size
		logger.Infof("Set limit test size to %d", limitTestSize)
	},
}

var THREAD_NUM = 8
var timeInterval = 1
var eachObjectSame = true
var limitTestSizeStr = "500GB"
var limitTestSize = int64(500 * 1024 * 1024 * 1024)

var benchmarkCmd = &cobra.Command{
	Use: "benchmark",
	Run: func(cmd *cobra.Command, args []string) {
		confPath := "./config.json"
		conf := config.DefaultConfig
		configUtil.Register(&conf, confPath)
		configUtil.ReadAll()
		testSize := []uint64{
			128,              // 128B
			4 * 1024,         // 4KB
			512 * 1024,       // 512KB
			1024 * 1024,      // 1MB
			4 * 1024 * 1024,  // 4MB
			16 * 1024 * 1024, // 16MB
			64 * 1024 * 1024, // 64MB
		}
		writePerformance := make([]float64, 0)
		readPerformance := make([]float64, 0)

		for _, size := range testSize {
			logger.Infof("Start benchmark with size %d", size)
			ctx, cancel := context.WithCancel(context.Background())
			var connector benchmark.Connector

			if len(args) > 0 && args[0] == "minio" {
				connector = benchmark.NewMinioConnector(ctx)
			} else {
				connector = benchmark.NewEcosConnector(ctx, &conf)
			}

			tester := benchmark.NewTester(ctx, connector, eachObjectSame)
			connector.Clear()
			go tester.TestWritePerformance(size, THREAD_NUM)
			time.Sleep(10 * time.Second)
			tester.Stop()
			writePerformance = append(writePerformance, tester.GetMean())
			cancel()

			logger.Infof("Start benchmark with size %d", size)
			ctx, cancel = context.WithCancel(context.Background())
			if len(args) > 0 && args[0] == "minio" {
				connector = benchmark.NewMinioConnector(ctx)
			} else {
				connector = benchmark.NewEcosConnector(ctx, &conf)
			}

			tester = benchmark.NewTester(ctx, connector, eachObjectSame)
			go tester.TestReadPerformance()
			time.Sleep(10 * time.Second)
			tester.Stop()
			readPerformance = append(readPerformance, tester.GetMean())
			cancel()
		}
		logger.Infof("Write performance:")
		logger.Infof("%v \n", writePerformance)
		logger.Infof("Read performance:")
		logger.Infof("%v \n", readPerformance)
	},
}

var runPutCmd = &cobra.Command{
	Use: "put {size}",
	Run: func(cmd *cobra.Command, args []string) {
		confPath := "./config.json"
		conf := config.DefaultConfig
		configUtil.Register(&conf, confPath)
		configUtil.ReadAll()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		connector := benchmark.NewEcosConnector(ctx, &conf)
		tester := benchmark.NewTester(ctx, connector, eachObjectSame)

		base := 1024 * 1024 // 1MB
		sizeStr := args[0]
		if sizeStr[len(sizeStr)-1] < '0' || sizeStr[len(sizeStr)-1] > '9' {
			if sizeStr[len(sizeStr)-1] == 'K' || sizeStr[len(sizeStr)-1] == 'k' {
				base = 1024
			}
			sizeStr = sizeStr[:len(sizeStr)-1]
		}
		size, _ := strconv.Atoi(sizeStr)
		size *= base

		go tester.TestWritePerformance(uint64(size), THREAD_NUM)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if tester.GetWriteBytes() > limitTestSize {
						logger.Infof("Reach limit test size %d", limitTestSize)
						tester.Stop()
						return
					}
				}
			}
		}()
		WaitSignal()
		logger.Infof("Test finished")
		tester.Stop()
	},
}

func WaitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	for _ = range c {
		return
	}
}

var runGetCmd = &cobra.Command{
	Use: "get",
	Run: func(cmd *cobra.Command, args []string) {
		confPath := "./config.json"
		conf := config.DefaultConfig
		configUtil.Register(&conf, confPath)
		configUtil.ReadAll()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.AddCommand(runPutCmd)
	runCmd.AddCommand(runGetCmd)
	runCmd.AddCommand(benchmarkCmd)

	runCmd.PersistentFlags().IntVarP(&THREAD_NUM, "thread num", "j", 1, "number of test threads")
	runCmd.PersistentFlags().IntVarP(&timeInterval, "network status interval", "i", 5, "interval of network status report")
	runCmd.PersistentFlags().BoolVarP(&eachObjectSame, "each object same", "s", true, "whether each object is same")
	runCmd.PersistentFlags().StringVarP(&limitTestSizeStr, "limit network data size", "l", "500GB", "test size")
	logger.Logger.SetLevel(logrus.InfoLevel)
}
