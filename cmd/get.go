package cmd

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/moio/minima/get"
)

var directory string
var archString string

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get [URL]",
	Short: "Downloads a repo",
	Long:  `Downloads a repository given its URL`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("Please specify one repo URL")
		}
		url := args[0]
		archs := make(map[string]bool)
		if archString != "all" {
			archStrings := strings.Split(archString, ",")
			for _, arch := range archStrings {
				archs[arch] = true
			}
		}
		log.Println("Processing " + url + "...")
		storage := get.NewStorage(directory)
		err := get.StoreRepo(url, storage, archs)
		if err != nil {
			log.Println("ERROR: " + err.Error())
			os.Exit(1)
		} else {
			log.Println("...done.")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&directory, "directory", "d", ".", "Destination directory to save the repo")
	getCmd.Flags().StringVarP(&archString, "archs", "a", "all", "Comma-separated list of archs to include")
}
