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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/updates"
	yaml "gopkg.in/yaml.v2"
)

// mufnsCmd represents the mufns command
var (
	update = &cobra.Command{
	Use:   "updates",
	Short: "searches all updates and syncs them to mirror",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		muFindAndSync()
	},
	}
	spitYamls bool
	justSearch bool
	thisMU string
	cleanup bool
	autoApprove bool
)

func init() {
	RootCmd.AddCommand(update)
	RootCmd.PersistentFlags().BoolVarP(&spitYamls, "yaml", "y", false, "flag that would trigger generating minima_obs_<Date>.yaml configs")
	RootCmd.PersistentFlags().BoolVarP(&justSearch, "search", "s", false, "flag that would trigger only looking for updates on OBS")
	RootCmd.PersistentFlags().StringVarP(&thisMU, "maintupdate", "m", "", "flag that consumes the name of an MU, like 'SUSE:Maintenance:Incident:ReleaseRequest'")
	RootCmd.PersistentFlags().BoolVarP(&cleanup, "cleanup", "k", false, "flag that triggers cleaning up the storage (from old MU channels)")
	RootCmd.PersistentFlags().BoolVar(&autoApprove, "auto-approve", false, "triggers automatic deleting of repositories")
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
		err = RemoveOldChannels(config, updateList, autoApprove)
		if err != nil {
			log.Fatalf("There is an error: %v", err)
		}
		log.Println("...done!")
	} else {
		if thisMU == "" {
			updateList, err = GetUpdatesAndChannels(config.OBS.Username, config.OBS.Password, justSearch)
			if err != nil {
				log.Fatalf("There is an error: %v", err)
			}
			config.HTTP = []HTTPRepoConfig{}
			for _, val := range updateList {
				config.HTTP = append(config.HTTP, val.Repositories...)
			}
		} else {	
			if mu := strings.Split(thisMU, ":"); len(mu) != 4 {
				log.Fatalf("Badly formatted MU. It must be SUSE:Maintenance:NUMBER:NUMBER")
			} else {
				
				a := Updates{}
				a.IncidentNumber = mu[2]
				a.ReleaseRequest = mu[3]
				mu := fmt.Sprintf("%s%s/", updates.DownloadIbsLink, a.IncidentNumber)
				a.Repositories, err = GetRepo(mu)
				config.HTTP = append(config.HTTP, a.Repositories...)
				if err != nil {
					 log.Fatalf("Something went wrong in Repo processing: %v\n", err)
				}
				updateList = append(updateList, a)
			}
		}
		var errorflag bool = false
		byteChunk, err := yaml.Marshal(config)
		if err != nil {
			log.Fatalf("There is an error: %v", err)
		}
		if spitYamls {
			//log.Printf("This is going to be added to config.HTTP: %v", config.HTTP)
			t := time.Now()
			err := ioutil.WriteFile(fmt.Sprintf("./minima_obs_%v-%v-%v-%v:%v.yaml", t.Year(), t.Month(), t.Local().Day(), t.Hour(), t.Minute()), byteChunk, 0644)
			if err != nil {
				log.Fatalf("There is an error: %v", err)
			}
			os.Exit(3)
		}
		if justSearch {
			for _, value := range updateList {
				fmt.Printf("INCIDENT: %v  RELEASEREQUEST: %v   PACKAGES: %v\n", value.IncidentNumber, value.ReleaseRequest, value.SRCRPMS )
			}
			os.Exit(3)
		} 
		syncers, err := syncersFromConfig(string(byteChunk))
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
	}
	//fmt.Printf("USERNAME: %s\nPASSWORD: %s\n", config.OBS.Username, config.OBS.Password)
	//time.Sleep(60 * time.Second)
}

func ProcWebChunk(val, maint string, register map[string]bool) (r map[string]bool, httpFormattedRepos []HTTPRepoConfig, err error) {
	var repo HTTPRepoConfig
	if regexp.MustCompile(`^SUSE`).FindString(val) != "" {
		val = maint + val
		if !register[val] {
			exists, err := updates.CheckWebPageExists(val)
			if err != nil {
				return nil, nil, err
			}
			if exists {
				register[val] = true
				repo.URL = val
				repo.Archs = []string{}
				reppo := make(chan HTTPRepoConfig)
				go func(rep HTTPRepoConfig) {
					rp, _ := ArchMage(rep)
					reppo <- rp
				}(repo)
				rep := <- reppo
				fmt.Println(rep)
				httpFormattedRepos = append(httpFormattedRepos, rep)
			} else {
				delete(register, val)
			}
		}
	}
	r = register
	return r, httpFormattedRepos, err
}


//---- This function checks that all architecture slice of a *HTTPRepoConfig is filled right
func ArchMage(repo HTTPRepoConfig) (HTTPRepoConfig, error) {
	archRegister := []string{"x86_64", "i586", "i686", "aarch64", "aarch64_ilp32", "ppc64le", "s390x", "src"}
	for _, arch := range archRegister {
		if strings.Contains(repo.URL, arch) {
			repo.Archs = append(repo.Archs, arch)
		} else {
			exists, err := updates.CheckWebPageExists(repo.URL + arch + "/")
			if err != nil {
				return repo, err
			}
			if exists {
				repo.Archs = append(repo.Archs, arch)
			}
		}
	}
	return repo, nil
}

func GetRepo(mu string) (httpFormattedRepos []HTTPRepoConfig, err error) {
	resp, err := http.Get(mu)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	repos := make(chan []HTTPRepoConfig)
	rr := make(chan map[string]bool)
	register := make(map[string]bool)
	for _, val := range strings.Split( string(body), "\"") {
		go func(vl, maint string, reg map[string]bool) {
			reg, slice, err := ProcWebChunk(vl, mu, reg)
			if err != nil {
				log.Fatalf("Error: %v\n", err)
			}
			repos <- slice
			rr <- reg
		}(val, mu, register)
		httpFormattedRepos = append(httpFormattedRepos, <-repos...)
		register = <-rr
	}
	return httpFormattedRepos, nil
}

func GetUpdatesAndChannels(usr, passwd string, justsearch bool) (updlist []Updates, err error){
	client := updates.NewClient(usr, passwd)
	rrs, err := client.GetReleaseRequests("qam-manager", "new,review")
	if err != nil {
		return updlist, fmt.Errorf("error while getting response from obs: %v", err)
	}
	for _, value := range rrs {
		var update Updates
		update.ReleaseRequest = value.Id

		for i:=0; i < len(value.Actions); i++ {
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
		}
		if err != nil {
			return updlist, fmt.Errorf("something went wrong in repo processing: %v", err)
		}
		updlist = append(updlist, update)
	}
	return updlist, err
}

func RemoveOldChannels(config Config, updates []Updates, deleteNow bool) (err error) {
	mappedUpdates := MakeAMap(updates)
	fmt.Printf("Mapped Updates: %s\n", mappedUpdates)
	switch config.Storage.Type {
	case "file":
		var muChannelList []string
		if _, err := os.Stat(filepath.Join(config.Storage.Path, "ibs/SUSE:/Maintenance:/")); os.IsNotExist(err) {
			log.Printf("This path: \"%s\" does not exist! Check your mirror...\n")
		} else {
			err = filepath.Walk(filepath.Join(config.Storage.Path, "ibs/SUSE:/Maintenance:/"), func(path string, info os.FileInfo, err error) error {
				if info.Name() == "repodata" {
					muChannelList = append(muChannelList, path)
				}
				if info.IsDir() {
					if files, err := ioutil.ReadDir(path); len(files) == 0  {
						if err != nil {
							log.Fatal(err)
						}
						log.Printf("Removing unused empty folders: %s\n", path)
						os.RemoveAll(filepath.Join(config.Storage.Path, "ibs/SUSE:/Maintenance:/", path))
					}
				}
				return nil
				})
		}
			if err != nil {
				return
			}
		//List of repositories that will be deleted: reposToDel
		var reposToDel map[string]bool
		fmt.Printf("MuchannelList: %v\n", muChannelList)
		for _, elem := range muChannelList {
				if regexp.MustCompile(`/\d{5,6}/`).FindString(elem) != "" {
					_, exists := mappedUpdates[strings.Replace(regexp.MustCompile(`/\d{5,6}/`).FindString(elem), "/", "", 10)]
					if !exists {
						if deleteNow {
							log.Printf("removing: %s...\n", elem)
							err = os.RemoveAll(elem)
							if err != nil {
								return
							}
						} else {
							reposToDel[elem] = true
						}
					}
				}
		}
		log.Println("Mirror will have following changes:")
		if reposToDel == nil {
			log.Println("...there is nothing to delete!")
		}
		for index, _ := range reposToDel {
			fmt.Printf("- %s", index)
		}
	case "s3":
	}
	return
}

func MakeAMap(updates []Updates) (updatesMap map[string]bool){
	updatesMap = make(map[string]bool)
	for _, elem := range updates {
		if elem.IncidentNumber == "" {
			elem.IncidentNumber = fmt.Sprintf("%s", elem.Repositories[0])
		}
		updatesMap[elem.IncidentNumber] = true
	}
	return
}

func CheckIFMapHasKey(updatesMap map[string]bool, prefix, elem string) (yes bool) {
	for key, _ := range updatesMap {
		if strings.Contains(key, strings.Replace(elem, prefix, "", 1)) {
			yes = true
		}
	}
	return
}

type Updates struct {
	IncidentNumber string
	ReleaseRequest string
	SRCRPMS        []string
	Products       string
	Repositories     []HTTPRepoConfig
}