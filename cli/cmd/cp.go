package cmd

import (
	"github.com/spf13/cobra"
	"strings"
)

var cpCmd = &cobra.Command{
	Use:   "cp [flags] [source] [destination]",
	Short: "Copy local path to ecos object",
	Run: func(cmd *cobra.Command, args []string) {
		readConfig(cmd, args)
		if strings.HasPrefix(args[1], "ecos://") {
			args[1] = strings.TrimPrefix(args[1], "ecos://")
			KeyPut(args[1], args[0])
		} else if strings.HasPrefix(args[0], "ecos://") {
			args[0] = strings.TrimPrefix(args[0], "ecos://")
			KeyGet(args[0], args[1])
		}
	},
	Args: cobra.ExactArgs(2),
}
