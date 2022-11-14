package cmd

import (
	"context"
	"ecos/client/config"
	"ecos/experiment/exp-client/benchmark"
	configUtil "ecos/utils/config"
	"github.com/spf13/cobra"
	"time"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run exp client",
	Long:  `Run exp client with default configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		confPath := "./config.json"
		conf := config.DefaultConfig
		configUtil.Register(&conf, confPath)
		configUtil.ReadAll()
		tester := benchmark.NewEcosTester(context.Background(), &conf)
		go tester.TestPerformance()
		time.Sleep(20 * time.Second)
		tester.Stop()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
