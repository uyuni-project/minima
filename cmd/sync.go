package cmd

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
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

			syncers, err := syncersFromConfig(cfgString)
			if err != nil {
				log.Fatal(err)
			}

			var errorflag bool = false
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

func syncersFromConfig(configString string) ([]*get.Syncer, error) {
	config, err := parseConfig(configString)
	if err != nil {
		return nil, err
	}
	// passing the flag value to a global variable in get package, to disables syncing of i586 and i686 rpms (usually inside x86_64)
	get.SkipLegacy = skipLegacyPackages
	// Go's default timeout for HTTP clients is 0, meaning there's no timeout
	// this can lead to connections hanging indefinitely
	http.DefaultClient.Timeout = time.Duration(config.TimeoutMinutes) * time.Minute

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

		httpRepoConfigs, err := get.SCCToHTTPConfigs(sccUrl, config.SCC.Username, config.SCC.Password, config.SCC.Repositories)
		if err != nil {
			return nil, err
		}
		config.HTTP = append(config.HTTP, httpRepoConfigs...)
	}

	syncers := []*get.Syncer{}
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
		switch config.Storage.Type {
		case "file":
			storage = get.NewFileStorage(filepath.Join(config.Storage.Path, filepath.FromSlash(repoURL.Path)))
		case "s3":
			storage, err = get.NewS3Storage(config.Storage.AccessKeyID, config.Storage.AccessKeyID, config.Storage.Region, config.Storage.Bucket+repoURL.Path)
			if err != nil {
				return nil, err
			}
		}
		syncers = append(syncers, get.NewSyncer(*repoURL, archs, storage))
	}

	return syncers, nil
}

func init() {
	RootCmd.AddCommand(syncCmd)
	// local flags
	syncCmd.Flags().StringVarP(&thisRepo, "repository", "r", "", "flag that can specifies a single repo (example: SLES11-SP4-Updates)")
	syncCmd.Flags().StringVarP(&archs, "arch", "a", "", "flag that specifies covered archs in the given repo")
	syncCmd.Flags().BoolVarP(&skipLegacyPackages, "nolegacy", "l", false, "flag that disables mirroring of i586 and i686 pkgs")
}
