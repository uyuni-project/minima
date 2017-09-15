package cmd

import (
	"log"

	"github.com/moio/minima/get"
	"github.com/smallfish/simpleyaml"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronizes repos from remote locations",
	Long: `Synchronizes content in repos to a directory or an S3 bucket.

  You can specify configuration in YAML either in a file or the MINIMA_CONFIG environment variable.

  An example minima.yaml is below:

    # filesystem directory example
    - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
      path: /tmp/minima/repo1

    # AWS S3 bucket example
    - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
      access_key_id: ACCESS_KEY_ID
      secret_access_key: SECRET_ACCESS_KEY
      region: us-east-1
      bucket: minima-bucket-key
      archs: [x86_64]
  `,
	Run: func(cmd *cobra.Command, args []string) {
		result, err := simpleyaml.NewYaml([]byte(cfgString))
		if err != nil {
			log.Fatal(err)
		}

		repoCount, err := result.GetArraySize()
		if err != nil {
			log.Fatal("Configuration parse error:", err)
		}

		var repoSyncers []*get.RepoSyncer
		for i := 0; i < repoCount; i++ {
			// common options
			repoNode := result.GetIndex(i)
			url, err := repoNode.Get("url").String()
			if err != nil {
				log.Fatal("Configuration parse error:", err)
			}

			archs := map[string]bool{}
			archsNode := repoNode.Get("archs")
			if archsNode.IsArray() {
				archStrings, err := archsNode.Array()
				if err != nil {
					log.Fatal("Configuration parse error:", err)
				}

				for _, archString := range archStrings {
					archs[archString.(string)] = true
				}
			}

			var storage get.Storage

			// file-specific
			pathNode := repoNode.Get("path")
			if pathNode.IsFound() {
				path, err := pathNode.String()
				if err != nil {
					log.Fatal("Configuration parse error: path is invalid")
				}
				log.Println(path)
				storage = get.NewFileStorage(path)
			} else {
				// s3-specific
				bucketNode := repoNode.Get("bucket")
				if bucketNode.IsFound() {
					bucket, err := bucketNode.String()
					if err != nil {
						log.Fatal("Configuration parse error: bucket is invalid")
					}

					region, err := repoNode.Get("region").String()
					if err != nil {
						log.Fatal("Configuration parse error: region invalid or not specified")
					}

					accessKeyID, err := repoNode.Get("access_key_id").String()
					if err != nil {
						log.Fatal("Configuration parse error: access_key_id invalid or not specified")
					}

					secretAccessKey, err := repoNode.Get("secret_access_key").String()
					if err != nil {
						log.Fatal("Configuration parse error: secret_access_key invalid or not specified")
					}

					storage, err = get.NewS3Storage(accessKeyID, secretAccessKey, region, bucket)
					if err != nil {
						log.Fatal(err)
					}
				} else {
					log.Fatal("Every repository must specify either path (filesystem storage) or bucket (AWS S3 storage)")
				}
			}

			repoSyncers = append(repoSyncers, get.NewRepoSyncer(url, archs, storage))
		}

		for _, repoSyncer := range repoSyncers {
			err := repoSyncer.StoreRepo()
			if err != nil {
				log.Fatal(err)
			} else {
				log.Println("...done.")
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(syncCmd)
}
