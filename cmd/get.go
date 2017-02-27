package cmd

import (
	"errors"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/moio/minima/get"
)

var directory string

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
		log.Println("Processing " + url + "...")
		storage := get.NewStorage(directory)
		err := get.StoreRepo(url, storage)
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
}
