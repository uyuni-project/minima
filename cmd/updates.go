/*
Copyright © 2021-2024 NAME HERE <EMAIL ADDRESS>

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
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/maint"
	yaml "gopkg.in/yaml.v2"
)

// updateCmd represents the mufns command
var (
	updateCmd = &cobra.Command{
		Use:   "updates",
		Short: "searches all updates and syncs them to mirror",
		Long: `Using the Build Service API, searches the updates repositories for the given MU and filters and syncs them to the mirror. 
		It is also possible to generate yaml file to be used for syncing such repositories, only search for the updates or do a cleanup
		of previous MUs repositories.
		`,
		Run: func(cmd *cobra.Command, args []string) {
			initConfig()
			muFindAndSync()
		},
	}
	spitYamls  bool
	justSearch bool
	thisMU     string
	cleanup    bool
	group      string
	states     string
)

func init() {
	RootCmd.AddCommand(updateCmd)
	// local flags
	updateCmd.Flags().BoolVarP(&spitYamls, "yaml", "y", false, "flag that would trigger generating minima_<BuildService>_<Date>.yaml configs")
	updateCmd.Flags().BoolVarP(&justSearch, "search", "s", false, "flag that would trigger only looking for updates on the Build Service")
	updateCmd.Flags().StringVarP(&thisMU, "maintupdate", "m", "", "flag that consumes the name of an MU, like 'SUSE:Maintenance:Incident:ReleaseRequest'")
	updateCmd.Flags().BoolVarP(&cleanup, "cleanup", "k", false, "flag that triggers cleaning up the storage (from old MU channels)")
	updateCmd.Flags().StringVarP(&group, "group", "g", "", "flag that applies a filter by group")
	updateCmd.Flags().StringVarP(&states, "states", "t", "new,review", "flag that applies a filter by state, multiple states must be separated by a comma (default 'new,review')")
}

func muFindAndSync() {
	config := Config{}
	updateList := []maint.Updates{}

	if err := yaml.Unmarshal([]byte(cfgString), &config); err != nil {
		log.Fatalf("Error reading configuration: %v", err)
	}

	client, err := maint.NewBuildServiceClient(config.BuildService.API, config.BuildService.Username, config.BuildService.Password)
	if err != nil {
		log.Fatal(err)
	}

	if cleanup {
		// DO CLEANUP - TO BE IMPLEMENTED
		log.Println("searching for outdated MU repos...")
		updateList, err := client.GetUpdatesAndChannels(group, states, true)
		if err != nil {
			log.Fatalf("Error searching for outdated MUs repos: %v", err)
		}

		err = client.RemoveOldChannels(config.Storage, updateList)
		if err != nil {
			log.Fatalf("Error removing old channels: %v", err)
		}
		log.Println("...done!")
	} else {
		if thisMU == "" {
			updateList, err := client.GetUpdatesAndChannels(group, states, justSearch)
			if err != nil {
				log.Fatalf("Error finding updates and channels: %v", err)
			}
			config.HTTP = []get.HTTPRepo{}
			for _, val := range updateList {
				config.HTTP = append(config.HTTP, val.Repositories...)
			}
		} else {
			if mu := strings.Split(thisMU, ":"); len(mu) != 4 {
				log.Fatalf("Badly formatted MU. It must be SUSE:Maintenance:NUMBER:NUMBER")
			} else {
				a := maint.Updates{}
				a.IncidentNumber = mu[2]
				a.ReleaseRequest = mu[3]

				repos, err := client.GetRepo(a.IncidentNumber)
				if err != nil {
					log.Fatalf("Something went wrong in MU %s repos processing: %v\n", mu, err)
				}
				a.Repositories = repos
				config.HTTP = append(config.HTTP, a.Repositories...)
				updateList = append(updateList, a)
			}
		}

		byteChunk, err := yaml.Marshal(config)
		if err != nil {
			log.Fatalf("Error marshalling config: %v", err)
		}

		if spitYamls {
			t := time.Now()
			err := os.WriteFile(fmt.Sprintf("./minima_%s_%v-%v-%v-%v:%v.yaml", config.BuildService.API, t.Year(), t.Month(), t.Local().Day(), t.Hour(), t.Minute()), byteChunk, 0644)
			if err != nil {
				log.Fatalf("Error writing file: %v", err)
			}
			os.Exit(3)
		}
		if justSearch {
			for _, value := range updateList {
				fmt.Printf("INCIDENT: %v  RELEASEREQUEST: %v   PACKAGES: %v\n", value.IncidentNumber, value.ReleaseRequest, value.SRCRPMS)
			}
			os.Exit(3)
		}

		syncers, err := syncersFromConfig(config)
		if err != nil {
			log.Fatal(err)
		}

		for _, syncer := range syncers {
			log.Printf("Processing repo: %s", syncer.URL.String())
			err := syncer.StoreRepo()
			if err != nil {
				log.Fatal(err)
			}
			log.Println("...done.")
		}
	}
}
