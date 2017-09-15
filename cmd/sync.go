package cmd

import (
	"fmt"
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
		syncers, err := syncersFromConfig(cfgString)
		if err != nil {
			log.Fatal(err)
		}
		for _, syncer := range syncers {
			err := syncer.StoreRepo()
			if err != nil {
				log.Fatal(err)
			} else {
				log.Println("...done.")
			}
		}
	},
}

func syncersFromConfig(config string) (result []*get.Syncer, err error) {
	yaml, err := simpleyaml.NewYaml([]byte(config))
	if err != nil {
		return
	}

	repoCount, err := yaml.GetArraySize()
	if err != nil {
		return nil, fmt.Errorf("Configuration parse error: top-level structure is not an array")
	}

	for i := 0; i < repoCount; i++ {
		// common options
		repoNode := yaml.GetIndex(i)
		url, err := repoNode.Get("url").String()
		if err != nil {
			return nil, fmt.Errorf("Configuration parse error in entry %d: every entry must have an url key", i+1)
		}

		archs := map[string]bool{}
		archsNode := repoNode.Get("archs")
		if archsNode.IsArray() {
			archStrings, err := archsNode.Array()
			if err != nil {
				return nil, fmt.Errorf("Configuration parse error for repo %s: archs should be an array", url)
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
				return nil, fmt.Errorf("Configuration parse error for repo %s: path is invalid", url)
			}
			log.Println(path)
			storage = get.NewFileStorage(path)
		} else {
			// s3-specific
			bucketNode := repoNode.Get("bucket")
			if bucketNode.IsFound() {
				bucket, err := bucketNode.String()
				if err != nil {
					return nil, fmt.Errorf("Configuration parse error for repo %s: bucket is invalid", url)
				}

				region, err := repoNode.Get("region").String()
				if err != nil {
					return nil, fmt.Errorf("Configuration parse error for repo %s: region invalid or not specified", url)
				}

				accessKeyID, err := repoNode.Get("access_key_id").String()
				if err != nil {
					return nil, fmt.Errorf("Configuration parse error for repo %s: access_key_id invalid or not specified", url)
				}

				secretAccessKey, err := repoNode.Get("secret_access_key").String()
				if err != nil {
					return nil, fmt.Errorf("Configuration parse error for repo %s: secret_access_key invalid or not specified", url)
				}

				storage, err = get.NewS3Storage(accessKeyID, secretAccessKey, region, bucket)
				if err != nil {
					log.Fatal(err)
				}
			} else {
				return nil, fmt.Errorf("Configuration parse error for repo %s: either path (filesystem storage) or bucket (AWS S3 storage) must be specified", url)
			}
		}

		result = append(result, get.NewSyncer(url, archs, storage))
	}
	return
}

func init() {
	RootCmd.AddCommand(syncCmd)
}
