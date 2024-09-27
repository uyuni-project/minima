/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"io"

	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/updates"
	yaml "gopkg.in/yaml.v2"
)

type Updates struct {
	IncidentNumber string
	ReleaseRequest string
	SRCRPMS        []string
	Products       string
	Repositories   []get.HTTPReposConfig
}

// package scoped array of all possible available archs to check for a repo
var architectures = [...]string{"x86_64", "i586", "i686", "aarch64", "aarch64_ilp32", "ppc64le", "s390x", "src"}

// updateCmd represents the updates command
var (
	updateCmd = &cobra.Command{
		Use:   "updates",
		Short: "searches all updates and syncs them to mirror",
		Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
		Run: func(cmd *cobra.Command, args []string) {
			initConfig()
			muFindAndSync()
		},
	}
	spitYamls  bool
	justSearch bool
	thisMU     string
	cleanup    bool
)

func init() {
	RootCmd.AddCommand(updateCmd)
	// local flags
	updateCmd.Flags().BoolVarP(&spitYamls, "yaml", "y", false, "flag that would trigger generating minima_obs_<Date>.yaml configs")
	updateCmd.Flags().BoolVarP(&justSearch, "search", "s", false, "flag that would trigger only looking for updates on OBS")
	updateCmd.Flags().StringVarP(&thisMU, "maintupdate", "m", "", "flag that consumes the name of an MU, like 'SUSE:Maintenance:Incident:ReleaseRequest'")
	updateCmd.Flags().BoolVarP(&cleanup, "cleanup", "k", false, "flag that triggers cleaning up the storage (from old MU channels)")
}

func muFindAndSync() {
	config := Config{}
	updateList := []Updates{}

	err := yaml.Unmarshal([]byte(cfgString), &config)
	if err != nil {
		log.Fatalf("There is an error: %v", err)
	}

	if cleanup {
		// DO CLEANUP - TO BE IMPLEMENTED
		log.Println("searching for outdated MU repos...")
		updateList, err = GetUpdatesAndChannels(config.OBS.Username, config.OBS.Password, true)
		if err != nil {
			log.Fatalf("There is an error: %v", err)
		}

		err = RemoveOldChannels(config, updateList)
		if err != nil {
			log.Fatalf("There is an error: %v", err)
		}
		log.Println("...done!")
		return
	}

	if thisMU == "" {
		updateList, err = GetUpdatesAndChannels(config.OBS.Username, config.OBS.Password, justSearch)
		if err != nil {
			log.Fatalf("There is an error: %v", err)
		}

		config.HTTP.Repositories = []get.HTTPReposConfig{}
		for _, val := range updateList {
			config.HTTP.Repositories = append(config.HTTP.Repositories, val.Repositories...)
		}
	} else {
		var mu []string
		if mu = strings.Split(thisMU, ":"); len(mu) != 4 {
			log.Fatalf("Badly formatted MU. It must be SUSE:Maintenance:NUMBER:NUMBER")
		}

		ups := Updates{}
		ups.IncidentNumber = mu[2]
		ups.ReleaseRequest = mu[3]
		muUrl := fmt.Sprintf("%s%s/", updates.DownloadIbsLink, ups.IncidentNumber)

		ups.Repositories, err = GetRepo(muUrl)
		config.HTTP.Repositories = append(config.HTTP.Repositories, ups.Repositories...)
		if err != nil {
			log.Fatalf("Something went wrong in Repo processing: %v\n", err)
		}
		updateList = append(updateList, ups)
	}

	byteChunk, err := yaml.Marshal(config)
	if err != nil {
		log.Fatalf("There is an error: %v", err)
	}

	if spitYamls {
		//log.Printf("This is going to be added to config.HTTP: %v", config.HTTP)
		t := time.Now()
		err := os.WriteFile(fmt.Sprintf("./minima_obs_%v-%v-%v-%v:%v.yaml", t.Year(), t.Month(), t.Local().Day(), t.Hour(), t.Minute()), byteChunk, 0644)
		if err != nil {
			log.Fatalf("There is an error: %v", err)
		}
		os.Exit(3)
	}

	if justSearch {
		for _, value := range updateList {
			fmt.Printf("INCIDENT: %v  RELEASEREQUEST: %v   PACKAGES: %v\n", value.IncidentNumber, value.ReleaseRequest, value.SRCRPMS)
		}
		os.Exit(3)
	}

	syncers, err := syncersFromConfig(string(byteChunk))
	if err != nil {
		log.Fatal(err)
	}

	for _, syncer := range syncers {
		log.Printf("Processing repo: %s", syncer.URL.String())
		err := syncer.StoreRepo()
		if err != nil {
			log.Fatal(err)
		} else {
			log.Println("...done.")
		}
	}
}

func ProcWebChunk(val, maint string, register map[string]bool) (map[string]bool, []get.HTTPReposConfig, error) {
	repos := []get.HTTPReposConfig{}

	if regexp.MustCompile(`^SUSE`).FindString(val) != "" {
		repoUrl := maint + val

		if !register[repoUrl] {
			exists, err := updates.CheckWebPageExists(repoUrl)
			if err != nil {
				return nil, nil, err
			}

			if exists {
				register[repoUrl] = true
				repo := get.HTTPReposConfig{
					URLs:  []string{repoUrl},
					Archs: []string{},
				}

				if err := ArchMage(&repo); err != nil {
					return nil, nil, err
				}
				fmt.Println(repos)
			} else {
				delete(register, val)
			}
		}
	}

	r := register
	return r, repos, nil
}

// ---- This function checks that all architecture slice of a *HTTPReposConfig is filled right
func ArchMage(repo *get.HTTPReposConfig) error {
	for _, arch := range architectures {
		for _, url := range repo.URLs {
			if strings.Contains(url, arch) {
				repo.Archs = append(repo.Archs, arch)
			} else {
				exists, err := updates.CheckWebPageExists(url + arch + "/")
				if err != nil {
					return err
				}

				if exists {
					repo.Archs = append(repo.Archs, arch)
				}
			}
		}
	}
	return nil
}

func GetRepo(mu string) ([]get.HTTPReposConfig, error) {
	resp, err := http.Get(mu)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	httpRepos := []get.HTTPReposConfig{}
	reposChan := make(chan []get.HTTPReposConfig)
	rr := make(chan map[string]bool)
	register := make(map[string]bool)

	for _, chunk := range strings.Split(string(body), "\"") {
		go func(c, maint string, reg map[string]bool) {
			reg, repos, err := ProcWebChunk(c, mu, reg)
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}

			reposChan <- repos
			rr <- reg
		}(chunk, mu, register)

		httpRepos = append(httpRepos, <-reposChan...)
		register = <-rr
	}

	return httpRepos, nil
}

func GetUpdatesAndChannels(usr, passwd string, justsearch bool) ([]Updates, error) {
	client := updates.NewClient(usr, passwd)
	rrs, err := client.GetReleaseRequests("qam-manager", "new,review")
	if err != nil {
		return nil, fmt.Errorf("error while getting response from obs: %v", err)
	}

	upds := []Updates{}
	for _, value := range rrs {
		var update Updates
		update.ReleaseRequest = value.Id

		for i := 0; i < len(value.Actions); i++ {
			if len(strings.Split(value.Actions[i].Target.Package, ".")) > 1 {
				update.IncidentNumber = strings.Split(value.Actions[i].Target.Package, ".")[1]
				if update.IncidentNumber != "" {
					break
				}
			}
		}

		for _, val := range value.Actions {
			if !strings.Contains(val.Target.Package, "patchinfo") && !(strings.Contains(val.Target.Package, "SLE") || strings.Contains(val.Target.Package, "Module")) {
				update.SRCRPMS = append(update.SRCRPMS, strings.Split(val.Target.Package, ".")[0])
			}
		}

		if !justsearch {
			update.Repositories, err = GetRepo(fmt.Sprintf("%s%s/", updates.DownloadIbsLink, update.IncidentNumber))
			if err != nil {
				return nil, fmt.Errorf("something went wrong in repo processing: %v", err)
			}
		}

		upds = append(upds, update)
	}
	return upds, err
}

func RemoveOldChannels(config Config, updates []Updates) (err error) {
	mappedUpdates := MakeAMap(updates)
	switch config.Storage.Type {
	case "file":
		var muChannelList []string
		err = filepath.Walk(filepath.Join(config.Storage.Path, "ibs/SUSE:/Maintenance:/"), func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				muChannelList = append(muChannelList, path)
			}
			return nil
		})
		if err != nil {
			return
		}
		//templ := regexp.MustCompile(`/\d{5,6}/`)
		for _, elem := range muChannelList {
			if regexp.MustCompile(`/\d{5,6}/`).FindString(elem) != "" {
				_, exists := mappedUpdates[strings.Replace(regexp.MustCompile(`/\d{5,6}/`).FindString(elem), "/", "", 10)]
				if !exists {
					log.Printf("removing: %s...\n", elem)
					err = os.RemoveAll(elem)
					if err != nil {
						return
					}
				}
			}
		}
	}
	return
}

func MakeAMap(updates []Updates) (updatesMap map[string]bool) {
	updatesMap = make(map[string]bool)
	for _, elem := range updates {
		updatesMap[elem.IncidentNumber] = true
	}
	return
}
