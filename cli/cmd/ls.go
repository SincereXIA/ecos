package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls [flags] [bucketName [prefix]]",
	Short: "List all objects in a bucket",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("ls all bucket not supported yet")
			return
		}
		if len(args) == 1 {
			KeyList(args[0], "")
			return
		}
		if len(args) == 2 {
			KeyList(args[0], args[1])
			return
		}
		fmt.Println("syntax error")
		return
	},
}
