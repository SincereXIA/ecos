package cmd

import (
	"ecos/client/config"
	"ecos/edge-node/infos"
	configUtil "ecos/utils/config"
	"ecos/utils/logger"
	"github.com/spf13/cobra"
)

var bucketCmd = &cobra.Command{
	Use:   "bucket {create}",
	Short: "operate bucket",
}

var bucketCreateCmd = &cobra.Command{
	Use:   "create {bucketName}",
	Short: "create bucket",
	Run: func(cmd *cobra.Command, args []string) {
		bucketCreate(args[0])
	},
	Args: cobra.ExactArgs(1),
}

func bucketCreate(bucketName string) {
	var conf config.ClientConfig
	_ = configUtil.GetConf(&conf)
	client := getClient()
	operator := client.GetVolumeOperator()
	bucketInfo := infos.GenBucketInfo(conf.Credential.GetUserID(), bucketName, conf.Credential.GetUserID())
	err := operator.CreateBucket(bucketInfo)
	if err != nil {
		logger.Errorf("create bucket failed, err: %v", err)
		return
	}
}
