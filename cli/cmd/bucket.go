package cmd

import "github.com/spf13/cobra"

var bucketCmd = &cobra.Command{
	Use:   "bucket {create}",
	Short: "operate bucket",
}
