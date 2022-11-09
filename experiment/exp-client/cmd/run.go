package cmd

import (
	"ecos/client/config"
	configUtil "ecos/utils/config"
	"github.com/spf13/cobra"
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

	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
