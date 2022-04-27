package cmd

import (
	"ecos/utils/logger"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster {describe}",
	Short: "Manage clusters",
}

var clusterDescribeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe ECOS cluster",
	Run: func(cmd *cobra.Command, args []string) {
		clusterDescribe()
	},
}

func clusterDescribe() {
	client := getClient()
	state, err := client.GetClusterOperator().State()
	if err != nil {
		logger.Errorf("Failed to get cluster state: %v", err)
		os.Exit(1)
	}
	fmt.Println(state)
}
