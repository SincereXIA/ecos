package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [flags] [bucketName]",
	Short: "List all objects in a bucket",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if len(args) == 0 {
			fmt.Println("ls all bucket not supported yet")
			return
		}
		KeyList(args[0])
	},
}
