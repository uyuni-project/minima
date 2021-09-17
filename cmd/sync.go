package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	yaml "gopkg.in/yaml.v2"
)

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
    #   repo_names:
    #     - SLES12-SP2-LTSS-Updates
    #   archs: [x86_64]
  `,
	Run: func(cmd *cobra.Command, args []string) {
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
thisRepo string
archs string
syncLegacyPackages bool
)

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
		JsonPath		string `yaml:"jsonpath"`
		ProjectID		string `yaml:"projectid"`
	}

	SCC struct {
		Username  string
		Password  string
		RepoNames []string `yaml:"repo_names"`
		Archs     []string
	}
	OBS struct {
		Username	string
		Password	string
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
	if storageType != "file" && storageType != "s3" && storageType != "gcp"{
		return nil, fmt.Errorf("configuration parse error: unrecognised storage type")
	}

	//---passing the flag value to a global variable in get package, to trigger syncing of i586 rpms inside x86_64
	get.Legacy = syncLegacyPackages

	if config.SCC.Username != "" {
		if thisRepo != "" {
			if archs == "" {
				archs = "x86_64"
			}
			config.SCC.RepoNames = []string{thisRepo}
			config.SCC.Archs = strings.Split(archs, ",")
		}

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
		switch storageType {
		case "file":
			storage = get.NewFileStorage(filepath.Join(config.Storage.Path, filepath.FromSlash(repoURL.Path)))
		case "s3":
			storage, err = get.NewS3Storage(config.Storage.AccessKeyID, config.Storage.AccessKeyID, config.Storage.Region, config.Storage.Bucket+repoURL.Path)
			if err != nil {
				return nil, err
			}
		case "gcp":
			storage, err = get.NewGCStorage(config.Storage.JsonPath, config.Storage.Region, config.Storage.ProjectID, config.Storage.Bucket)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, get.NewSyncer(*repoURL, archs, storage))
	}
	if cleanup {
		RemoveOldChannelsFromFileStorage(config)
	}
	return
}

func init() {
	RootCmd.AddCommand(syncCmd)
	RootCmd.PersistentFlags().StringVarP(&thisRepo, "repository", "r", "", "flag that can specifies a single repo (example: SLES11-SP4-Updates)")
	RootCmd.PersistentFlags().StringVarP(&archs, "arch", "a", "", "flag that specifies covered archs in the given repo")
	RootCmd.PersistentFlags().BoolVarP(&syncLegacyPackages, "legacypackages", "l", false, "flag that triggers mirroring of i586 pkgs in x86_64 repos")
}

func RemoveOldChannelsFromFileStorage (config Config) (err error) {
		// DO CLEANUP - TO BE IMPLEMENTED
		log.Println("searching for outdated repositories...")
		//fmt.Printf("List of Repos from config: %s  ---> %s\n", config.SCC.RepoNames, config.HTTP)
		mappedRepos := make(map[string]bool)
		var urlink *url.URL
		for _, elem := range config.HTTP {
			urlink, err = url.Parse(elem.URL)
			if err != nil {
				panic(err)
			}
			mappedRepos[filepath.Join(config.Storage.Path, urlink.Path)] = true
		}
			//fmt.Printf("MAPPED REPOS: %v\n", mappedRepos)
			path := config.Storage.Path
			muChannelList := make(map[string]bool)
			filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					if info.Name() == "repodata" {
						muChannelList[strings.Replace(path, "/repodata", "", 1)] = true
						return nil
					}
					if files, err := ioutil.ReadDir(path); len(files) == 0 && path != config.Storage.Path {
						if err != nil {
							log.Fatal(err)
						}
						log.Printf("Removing unused empty folders: %s\n", path)
						os.RemoveAll(path)
					}
				}
				return nil
			})
			//fmt.Printf("CHANNEL LIST: %v\n", muChannelList)
			for ind, _ := range muChannelList {
				if mappedRepos[ind] {
					log.Printf("Repo %s is registered...\n", ind)
				} else {
					log.Printf("Repo %s is not registered in the yaml file...", ind)
					if autoApprove {
						log.Printf("Removing repo %s ...\n", ind)
						os.RemoveAll(ind)
					} else {
						prompt := promptui.Select{
							Label: fmt.Sprintf("Delete repo: %s ??? [Yes/No]", ind),
							Items: []string{"Yes", "No"},
						}
						_, result, err := prompt.Run()
						if err != nil {
							log.Fatalf("Prompt failed %v\n", err)
						}
						if result == "Yes" {
							log.Printf("Removing repo: %s ...\n", ind)
							os.RemoveAll(ind)
						} else {
							log.Printf("Keeping repo: %s ...\n", ind)
						}
					}
				}
			}
		log.Println("...done!")
	return
}
