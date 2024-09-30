package cmd

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/updates"
	yaml "gopkg.in/yaml.v2"
)

const sccUrl = "https://scc.suse.com"

// syncCmd represents the sync command
var (
	syncCmd = &cobra.Command{
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
	  repositories:
        - urls:
	  	  - http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
          archs: [x86_64]

    # optional section to download repos from SCC
    # scc:
    #   username: UC7
    #   password: INSERT_PASSWORD_HERE
    #   repositories:
	#     - names:
    #       - SLE-Product-SLES15-SP5-Pool
	#       - SLE-Product-SLES15-SP5-Updates
    #       archs: [x86_64]
  `,
		Run: func(cmd *cobra.Command, args []string) {
			initConfig()

			var errorflag bool = false
			syncers, err := syncersFromConfig(cfgString)
			if err != nil {
				log.Fatal(err)
				errorflag = true
			}
			for _, syncer := range syncers {
				log.Printf("Processing repo: %s", syncer.URL.String())
				err := syncer.StoreRepo()
				if err != nil {
					log.Println(err)
					errorflag = true
				} else {
					log.Println("...done.")
				}
			}
			if errorflag {
				os.Exit(1)
			}
		},
	}
	thisRepo           string
	archs              string
	syncLegacyPackages bool
)

// Config maps the configuration in minima.yaml
type Config struct {
	Storage get.StorageConfig
	SCC     get.SCC
	OBS     updates.OBS
	HTTP    get.HTTP
}

func syncersFromConfig(configString string) ([]*get.Syncer, error) {
	config, err := parseConfig(configString)
	if err != nil {
		return nil, err
	}
	//---passing the flag value to a global variable in get package, to trigger syncing of i586 rpms inside x86_64
	get.Legacy = syncLegacyPackages

	if config.SCC.Username != "" {
		if thisRepo != "" {
			if archs == "" {
				archs = "x86_64"
			}
			config.SCC.Repositories = []get.SCCReposConfig{
				{
					Names: []string{thisRepo},
					Archs: strings.Split(archs, ","),
				},
			}
		}

		httpReposConfigs, err := get.SCCToHTTPConfigs(sccUrl, config.SCC.Username, config.SCC.Password, config.SCC.Repositories)
		if err != nil {
			return nil, err
		}
		config.HTTP.Repositories = append(config.HTTP.Repositories, httpReposConfigs...)
	}

	syncers := []*get.Syncer{}
	for _, reposConfig := range config.HTTP.Repositories {
		for _, repoURL := range reposConfig.URLs {
			parsedURL, err := url.Parse(repoURL)
			if err != nil {
				return nil, err
			}

			archs := map[string]bool{}
			for _, archString := range reposConfig.Archs {
				archs[archString] = true
			}

			var storage get.Storage
			switch config.Storage.Type {
			case "file":
				storage = get.NewFileStorage(filepath.Join(config.Storage.Path, filepath.FromSlash(parsedURL.Path)))
			case "s3":
				storage, err = get.NewS3Storage(config.Storage.AccessKeyID, config.Storage.AccessKeyID, config.Storage.Region, config.Storage.Bucket+parsedURL.Path)
				if err != nil {
					return nil, err
				}
			}
			syncers = append(syncers, get.NewSyncer(*parsedURL, archs, storage))
		}
	}

	return syncers, nil
}

func parseConfig(configString string) (Config, error) {
	config := Config{}
	if err := yaml.Unmarshal([]byte(configString), &config); err != nil {
		return config, fmt.Errorf("configuration parse error: %v", err)
	}

	storageType := config.Storage.Type
	if storageType != "file" && storageType != "s3" {
		return config, fmt.Errorf("configuration parse error: unrecognised storage type")
	}
	return config, nil
}

func init() {
	RootCmd.AddCommand(syncCmd)
	// local flags
	syncCmd.Flags().StringVarP(&thisRepo, "repository", "r", "", "flag that can specifies a single repo (example: SLES11-SP4-Updates)")
	syncCmd.Flags().StringVarP(&archs, "arch", "a", "", "flag that specifies covered archs in the given repo")
	syncCmd.Flags().BoolVarP(&syncLegacyPackages, "legacypackages", "l", false, "flag that triggers mirroring of i586 pkgs in x86_64 repos")
}
