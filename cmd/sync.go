package cmd

import (
	"fmt"
	"log"
	"net/url"
	"path/filepath"

	"github.com/moio/minima/get"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronizes repos from remote locations",
	Long: `Synchronizes content in repos to a directory or an S3 bucket.

  You can specify configuration in YAML either in a file or the MINIMA_CONFIG environment variable.

  A directory-based example minima.yaml is below:
    storage:
      type: file
      path: /srv/mirror

    http:
      - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/

  An s3-based example minima.yaml is below:
    storage:
      type: s3
      access_key_id: ACCESS_KEY_ID
      secret_access_key: SECRET_ACCESS_KEY
      region: us-east-1
      bucket: minima-bucket-key

    - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
      archs: [x86_64]
  `,
	Run: func(cmd *cobra.Command, args []string) {
		syncers, err := syncersFromConfig(cfgString)
		if err != nil {
			log.Fatal(err)
		}
		for _, syncer := range syncers {
			log.Printf("Processing repo: %s", syncer.Url)
			err := syncer.StoreRepo()
			if err != nil {
				log.Println(err)
			} else {
				log.Println("...done.")
			}
		}
	},
}

// Config maps the configuraiton in minima.yaml
type Config struct {
	Storage struct {
		Type string
		// file-specific
		Path string
		// s3-specific
		AccessKeyID     string `yaml:"access_key_id"`
		SecretAccessKey string `yaml:"secret_access_key"`
		Region          string
		Bucket          string
	}
	HTTP []struct {
		URL   string
		Archs []string
	}
}

func syncersFromConfig(configString string) (result []*get.Syncer, err error) {
	config := Config{}
	err = yaml.Unmarshal([]byte(configString), &config)

	storageType := config.Storage.Type
	if storageType != "file" && storageType != "s3" {
		return nil, fmt.Errorf("Configuration parse error: unrecognised storage type")
	}

	for _, httpRepo := range config.HTTP {
		repoURL, err := url.Parse(httpRepo.URL)
		if err != nil {
			return nil, err
		}

		archs := map[string]bool{}
		for _, archString := range httpRepo.Archs {
			archs[archString] = true
		}

		var storage get.Storage
		if storageType == "file" {
			storage = get.NewFileStorage(filepath.Join(config.Storage.Path, filepath.FromSlash(repoURL.Path)))
		}
		if storageType == "s3" {
			storage, err = get.NewS3Storage(config.Storage.AccessKeyID, config.Storage.AccessKeyID, config.Storage.Region, config.Storage.Bucket+repoURL.Path)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, get.NewSyncer(httpRepo.URL, archs, storage))
	}

	return
}

func init() {
	RootCmd.AddCommand(syncCmd)
}
