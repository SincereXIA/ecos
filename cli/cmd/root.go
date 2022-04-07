package cmd

import (
	"ecos/client/config"
	configUtil "ecos/utils/config"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ECOS-client",
	Short: "CLI for ecos-client, support put/get/delete key in ecos",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.AddCommand(keyCmd)
	keyCmd.AddCommand(keyPutCmd)
	keyCmd.AddCommand(keyGetCmd)
	keyCmd.AddCommand(keyListCmd)

	rootCmd.AddCommand(cpCmd)
	rootCmd.AddCommand(lsCmd)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func readConfig(cmd *cobra.Command, _ []string) {
	confPath := cmd.Flag("config").Value.String()
	conf := config.DefaultConfig
	configUtil.Register(&conf, confPath)
	configUtil.ReadAll()
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.client.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().StringP("config", "c", "./client.json",
		"config file path")
}
