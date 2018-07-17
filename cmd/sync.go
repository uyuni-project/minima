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

  An example minima.yaml is below:

    storage:
      type: file
      path: /srv/mirror
      # uncomment to save to an AWS S3 bucket instead of the filesystem
      # type: s3
      # access_key_id: ACCESS_KEY_ID
      # secret_access_key: SECRET_ACCESS_KEY
      # region: us-east-1
      # bucket: minima-bucket-key

    http:
      - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
        archs: [x86_64]

    # optional section to download repos from SCC
    # scc:
    #   username: UC7
    #   password: INSERT_PASSWORD_HERE
    #   repo_names:
    #     - SLES12-SP2-LTSS-Updates
    #   archs: [x86_64]
  `,
	Run: func(cmd *cobra.Command, args []string) {
		syncers, err := syncersFromConfig(cfgString)
		if err != nil {
			log.Fatal(err)
		}
		for _, syncer := range syncers {
			log.Printf("Processing repo: %s", syncer.URL.String())
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
	SCC struct {
		Username  string
		Password  string
		RepoNames []string `yaml:"repo_names"`
		Archs     []string
	}
	HTTP []HTTPRepoConfig
}

// HTTPRepoConfig defines the configuration of an HTTP repo
type HTTPRepoConfig struct {
	URL   string
	Archs []string
}

func syncersFromConfig(configString string) (result []*get.Syncer, err error) {
	config := Config{}
	err = yaml.Unmarshal([]byte(configString), &config)

	storageType := config.Storage.Type
	if storageType != "file" && storageType != "s3" {
		return nil, fmt.Errorf("Configuration parse error: unrecognised storage type")
	}

	if config.SCC.Username != "" {
		httpURLs, err := get.SCCURLs("https://scc.suse.com", config.SCC.Username, config.SCC.Password, config.SCC.RepoNames, config.SCC.Archs)
		if err != nil {
			return nil, err
		}

		for _, httpURL := range httpURLs {
			config.HTTP = append(config.HTTP, HTTPRepoConfig{httpURL, config.SCC.Archs})
		}
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

		result = append(result, get.NewSyncer(*repoURL, archs, storage))
	}

	return
}

func init() {
	RootCmd.AddCommand(syncCmd)
}
