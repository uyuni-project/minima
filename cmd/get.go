package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/moio/minima/get"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [URL]",
	Short: "Downloads a repo",
	Long:  `Downloads a repository given its URL`,
	Run: func(cmd *cobra.Command, args []string) {
		status := 0
		for i := 0; i < len(args); i++ {
			url := args[i]
			log.Println("Processing " + url + "...")
			resp, err := get.Get(url)
			if err != nil {
				log.Println("ERROR: " + err.Error())
				status = 1
			} else {
				log.Println(resp)
			}
		}
		os.Exit(status)
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
}
