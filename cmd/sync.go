package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/maint"
	"github.com/uyuni-project/minima/scc"
	"github.com/uyuni-project/minima/storage"
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
      - url: http://download.opensuse.org/repositories/myrepo1/openSUSE_Leap_42.3/
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
	skipLegacyPackages bool
)

// Config maps the configuration in minima.yaml
type Config struct {
	Storage      storage.StorageConfig
	SCC          scc.SCC
	BuildService maint.BuildServiceCredentials
	HTTP         []get.HTTPRepoConfig
}

func syncersFromConfig(configString string) ([]*get.Syncer, error) {
	config, err := parseConfig(configString)
	if err != nil {
		return nil, err
	}
	//---passing the flag value to a global variable in get package, to disables syncing of i586 and i686 rpms (usually inside x86_64)
	get.SkipLegacy = skipLegacyPackages

	if config.SCC.Username != "" {
		if thisRepo != "" {
			if archs == "" {
				archs = "x86_64"
			}
			config.SCC.Repositories = []scc.SCCRepos{
				{
					Names: []string{thisRepo},
					Archs: strings.Split(archs, ","),
				},
			}
		}

		httpRepoConfigs, err := scc.SCCToHTTPConfigs(sccUrl, config.SCC.Username, config.SCC.Password, config.SCC.Repositories)
		if err != nil {
			return nil, err
		}
		config.HTTP = append(config.HTTP, httpRepoConfigs...)
	}

	return get.SyncersFromHTTPRepos(config.HTTP, config.Storage)
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
	syncCmd.Flags().BoolVarP(&skipLegacyPackages, "nolegacy", "l", false, "flag that disables mirroring of i586 and i686 pkgs")
}
