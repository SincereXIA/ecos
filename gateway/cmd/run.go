package cmd

import (
	"ecos/gateway/router"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
	prometheusMetrics "github.com/deathowl/go-metrics-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rcrowley/go-metrics"
	"github.com/spf13/cobra"
	"time"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "run Ecos S3 Gateway Server",
	Long:  `Run Ecos S3 Gateway with default configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		confPath := cmd.Flag("config").Value.String()
		conf := router.DefaultConfig
		configUtil.Register(&conf, confPath)
		configUtil.ReadAll()
		router := router.NewRouter(conf)
		prometheusClient := prometheusMetrics.NewPrometheusProvider(
			metrics.DefaultRegistry, "ecos", "gateway", prometheus.DefaultRegisterer, 1*time.Second)
		go prometheusClient.UpdatePrometheusMetrics()
		go func() {
			for {
				time.Sleep(1 * time.Second)
				err := push.New("http://gateway.prometheus.sums.top", "ecos-gateway").Gatherer(prometheus.DefaultGatherer).Add()
				if err != nil {
					logger.Warningf("failed to push metrics to prometheus: %s", err)
				}
			}
		}()
		err := router.Run(":" + cmd.Flag("port").Value.String())
		if err != nil {
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	runCmd.Flags().Int16P("port", "p", 8080, "Port to listen on")
}
