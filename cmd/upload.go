package cmd

import (
	"errors"
	"log"
	"os"

	"github.com/moio/minima/get"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload [URL]",
	Short: "Uploads a repo to Amazon S3",
	Long:  "Uploads a repo from an URL to an Amazon S3 bucket",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		if len(args) != 1 {
			return errors.New("Please specify one repo URL")
		}
		url := args[0]
		if !viper.IsSet("access_key_id") || !viper.IsSet("secret_access_key") || !viper.IsSet("region") || !viper.IsSet("bucket") {
			cmd.SilenceErrors = true
			return errors.New(`Error: configuration values are missing. Note that the following configuration values must be set:

        access_key_id: your AWS Access Key ID
        secret_access_key: your AWS Secret Access Key
        region: the AWS region the S3 bucket is located in (eg. us-east-1)
        bucket: the S3 bucket ID

        You can set those either via the config file or (uppercased) environment variables.
      `)
		}
		log.Println("Processing " + url + "...")
		storage, err := get.NewS3Storage(viper.GetString("access_key_id"), viper.GetString("secret_access_key"), viper.GetString("region"), viper.GetString("bucket"))
		if err != nil {
			return
		}

		archs := map[string]bool{}
		reposyncer := get.NewRepoSyncer(url, archs, storage)
		err = reposyncer.StoreRepo()
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
	RootCmd.AddCommand(uploadCmd)
}
