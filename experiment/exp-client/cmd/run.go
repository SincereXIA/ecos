package cmd

import (
	"context"
	"ecos/client/config"
	"ecos/experiment/exp-client/benchmark"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
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
}

var benchmarkCmd = &cobra.Command{
	Use: "benchmark",
	Run: func(cmd *cobra.Command, args []string) {
		confPath := "./config.json"
		conf := config.DefaultConfig
		configUtil.Register(&conf, confPath)
		configUtil.ReadAll()
		testSize := []uint64{
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

			tester := benchmark.NewTester(ctx, connector)
			connector.Clear()
			go tester.TestWritePerformance(size)
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

			tester = benchmark.NewTester(ctx, connector)
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
		tester := benchmark.NewTester(ctx, connector)

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

		go tester.TestWritePerformance(uint64(size))
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
}
