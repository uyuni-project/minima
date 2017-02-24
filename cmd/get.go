package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [URL]",
	Short: "Downloads a repo",
	Long:  `Downloads a repository given its URL`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("get")
		for i := 0; i < len(args); i++ {
			fmt.Print(" " + args[i])
		}
		fmt.Println()
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
}
