package cmd

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	ecosIO "ecos/client/io"
	"ecos/utils/logger"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"io"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var keyCmd = &cobra.Command{
	Use:   "key {put | list}",
	Short: "operate object in ecos by key",
}

var keyPutCmd = &cobra.Command{
	Use:   "put ecos_key local_path",
	Short: "put a local file as an object in ecos, remote key: ecos_key, local file path: local_path",
	Run: func(cmd *cobra.Command, args []string) {
		KeyPut(args[0], args[1])
	},
	Args: cobra.ExactArgs(2),
}

var keyListCmd = &cobra.Command{
	Use:   "list bucket_name",
	Short: "list objects in ecos bucket",
	Run: func(cmd *cobra.Command, args []string) {
		KeyList(args[0])
	},
	Args: cobra.ExactArgs(1),
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
	factory := ecosIO.NewEcosIOFactory(config.DefaultConfig, "root", "default")
	writer := factory.GetEcosWriter(key)
	fi, err := os.Open(path)
	if err != nil {
		logger.Errorf("open file: %v, fail: %v", path, err)
		os.Exit(1)
	}
	defer func(fi *os.File) {
		err = fi.Close()
		if err != nil {
			logger.Errorf("close file: %v, fail: %v", path, err)
		}
	}(fi)
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

func KeyList(bucketName string) {
	ctx := context.Background()
	c, err := client.New(config.DefaultConfig)
	if err != nil {
		logger.Errorf("create client fail: %v", err)
		os.Exit(1)
	}
	objects, err := c.ListObjects(ctx, bucketName)
	tableStyle := table.StyleDefault
	tableStyle.Options = table.Options{
		DrawBorder:      false,
		SeparateColumns: false,
		SeparateFooter:  false,
		SeparateHeader:  true,
		SeparateRows:    false,
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(tableStyle)
	t.AppendHeader(table.Row{"Size", "LastModified", "Key"})
	for _, object := range objects {
		t.AppendRow(table.Row{fmt.Sprintf("%d", object.ObjSize), object.UpdateTime.Format("2006-01-02 15:04:05"), object.ObjId})
	}
	t.Render()
}
