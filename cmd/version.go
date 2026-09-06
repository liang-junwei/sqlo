package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const Version = "0.2.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("sqlo version %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
