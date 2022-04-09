package cmd

import (
	"context"
	"ecos/client"
	"ecos/client/config"
	ecosIO "ecos/client/io"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"io"
	"os"
)

// rootCmd represents the base command when called without any subcommands
var keyCmd = &cobra.Command{
	Use:   "key {put | get | list | delete | describe}",
	Short: "operate object in ecos by key",
}

var keyPutCmd = &cobra.Command{
	Use:   "put [bucketName] ecos_key local_path",
	Short: "put a local file as an object in ecos, remote key: ecos_key, local file path: local_path",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if len(args) == 3 {
			KeyPut(args[0], args[1], args[2])
		} else {
			KeyPut("default", args[0], args[1])
		}
	},
	Args: cobra.RangeArgs(2, 3),
}

var keyGetCmd = &cobra.Command{
	Use:   "get [bucketName] ecos_key local_path",
	Short: "get an remote object in ecos, remote key: ecos_key, local file path: local_path",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if len(args) == 3 {
			KeyGet(args[0], args[1], args[2])
		} else {
			KeyGet("default", args[0], args[1])
		}
	},
	Args: cobra.RangeArgs(2, 3),
}

var keyListCmd = &cobra.Command{
	Use:   "list bucket_name",
	Short: "list objects in ecos bucket",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if len(args) == 1 {
			KeyList(args[0])
		} else {
			KeyList("default")
		}
	},
	Args: cobra.RangeArgs(0, 1),
}

var KeyDeleteCmd = &cobra.Command{
	Use:   "delete [bucketName] ecos_key",
	Short: "delete an object in ecos, remote key: ecos_key",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if len(args) == 2 {
			KeyDelete(args[0], args[1])
		} else {
			KeyDelete("default", args[0])
		}
	},
	Args: cobra.RangeArgs(1, 2),
}

var keyDescribeCmd = &cobra.Command{
	Use:   "describe [bucketName] ecos_key",
	Short: "describe an object in ecos, remote key: ecos_key",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if len(args) == 2 {
			KeyDescribe(args[0], args[1])
		} else {
			KeyDescribe("default", args[0])
		}
	},
	Args: cobra.RangeArgs(1, 2),
}

func KeyGet(bucketName string, key string, path string) {
	c := getClient()
	factory := c.GetIOFactory(bucketName)
	reader := factory.GetEcosReader(key)

	file, err := os.Create(path)
	if err != nil {
		logger.Errorf("create file: %v error: %v", path, err)
		os.Exit(1)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logger.Errorf("close file: %v error: %v", path, err)
			os.Exit(1)
		}
	}(file)

	_, err = io.Copy(file, reader)
	if err != nil {
		logger.Errorf("get object: %v error: %v", key, err)
		os.Exit(1)
	}
}

func KeyPut(bucketName string, key string, path string) {
	var conf config.ClientConfig
	_ = configUtil.GetConf(&conf)
	factory := ecosIO.NewEcosIOFactory(&conf, "root", bucketName)
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
	c := getClient()
	objects, err := c.ListObjects(ctx, bucketName)
	if err != nil {
		logger.Errorf("list objects fail: %v", err)
	}
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

func KeyDelete(bucketName, key string) {
	var conf config.ClientConfig
	_ = configUtil.GetConf(&conf)
	c := getClient()
	volume := c.GetVolumeOperator()
	bucket, err := volume.Get(bucketName)
	if err != nil {
		logger.Errorf("get bucket fail: %v", err)
		os.Exit(1)
	}
	err = bucket.Remove(key)
	if err != nil {
		logger.Errorf("delete object: %v fail: %v", err)
		os.Exit(1)
	}
}

func KeyDescribe(bucketName, key string) {
	var conf config.ClientConfig
	_ = configUtil.GetConf(&conf)
	c, err := client.New(&conf)
	if err != nil {
		logger.Errorf("create client fail: %v", err)
		os.Exit(1)
	}
	operator := c.GetVolumeOperator()
	bucket, err := operator.Get(bucketName)
	if err != nil {
		logger.Errorf("get bucket fail: %v", err)
		os.Exit(1)
	}
	obj, err := bucket.Get(key)
	if err != nil {
		logger.Errorf("get object fail: %v", err)
		os.Exit(1)
	}
	state, err := obj.State()
	if err != nil {
		logger.Errorf("get object state fail: %v", err)
		os.Exit(1)
	}
	fmt.Println(state)
}

func getClient() *client.Client {
	var conf config.ClientConfig
	_ = configUtil.GetConf(&conf)
	c, err := client.New(&conf)
	if err != nil {
		logger.Errorf("create client fail: %v", err)
		os.Exit(1)
	}
	return c
}
