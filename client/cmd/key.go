package cmd

import (
	"ecos/client/config"
	"ecos/client/object"
	"ecos/utils/logger"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var keyCmd = &cobra.Command{
	Use:   "key {put}",
	Short: "operate object in ecos by key",
}

var keyPutCmd = &cobra.Command{
	Use:   "put ecos_key local_path",
	Short: "put a local file as an object in ecos, remote key: ecos_key, local file path: local_path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("key put cmd")
		KeyPut(args[0], args[1])
	},
	Args: cobra.ExactArgs(2),
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.client.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func KeyPut(key string, path string) {
	factory := object.NewEcosIOFactory(config.DefaultConfig)
	writer := factory.GetEcosWriter(key)
	fi, err := os.Open(path)
	if err != nil {
		logger.Errorf("open file: %v, fail: %v", path, err)
		os.Exit(1)
	}
	defer fi.Close()
	_, err = io.Copy(&writer, fi)
	if err != nil {
		logger.Errorf("put key fail: %v", err)
		os.Exit(1)
	}
	err = writer.Close()
	if err != nil {
		logger.Errorf("put key fail: %v", err)
		os.Exit(1)
	}
	logger.Infof("put key: %v success", key)
}
