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
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/maint"
	yaml "gopkg.in/yaml.v2"
)

type Updates struct {
	IncidentNumber string
	ReleaseRequest string
	SRCRPMS        []string
	Products       string
	Repositories   []get.HTTPRepo
}

// package scoped array of all possible available archs to check for a repo
var architectures = [...]string{"x86_64", "i586", "i686", "aarch64", "aarch64_ilp32", "ppc64le", "s390x", "src"}

// package scoped Thread-safe Map used as cache to check the existence of repositories
var register sync.Map

// mufnsCmd represents the mufns command
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
		log.Fatalf("Error reading configuration: %v", err)
	}

	if cleanup {
		// DO CLEANUP - TO BE IMPLEMENTED
		log.Println("searching for outdated MU repos...")
		updateList, err = GetUpdatesAndChannels(config.BuildService.Username, config.BuildService.Password, true)
		if err != nil {
			log.Fatalf("Error searching for outdated MUs repos: %v", err)
		}
		err = RemoveOldChannels(config, updateList)
		if err != nil {
			log.Fatalf("Error removing old channels: %v", err)
		}
		log.Println("...done!")
	} else {
		if thisMU == "" {
			updateList, err = GetUpdatesAndChannels(config.BuildService.Username, config.BuildService.Password, justSearch)
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
				a := Updates{}
				a.IncidentNumber = mu[2]
				a.ReleaseRequest = mu[3]
				mu := fmt.Sprintf("%s%s/", maint.DownloadIbsLink, a.IncidentNumber)

				a.Repositories, err = GetRepo(http.DefaultClient, mu)
				if err != nil {
					log.Fatalf("Something went wrong in MU %s repos processing: %v\n", mu, err)
				}
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
			err := os.WriteFile(fmt.Sprintf("./minima_obs_%v-%v-%v-%v:%v.yaml", t.Year(), t.Month(), t.Local().Day(), t.Hour(), t.Minute()), byteChunk, 0644)
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

		syncers, err := syncersFromConfig(string(byteChunk))
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

// ProcWebChunk retrieves repositories data for a product target in a MU
func ProcWebChunk(client *http.Client, product, maint string) ([]get.HTTPRepo, error) {
	httpFormattedRepos := []get.HTTPRepo{}
	repo := get.HTTPRepo{
		Archs: []string{},
	}
	repoUrl := maint + product

	_, ok := register.Load(repoUrl)
	if !ok {
		exists, err := get.CheckWebPageExists(client, repoUrl)
		if err != nil {
			return nil, err
		}
		register.Store(repoUrl, exists)

		if exists {
			repo.URL = repoUrl
			if err := ArchMage(client, &repo); err != nil {
				return nil, err
			}
			fmt.Println(repo)
			httpFormattedRepos = append(httpFormattedRepos, repo)
		}
	}
	return httpFormattedRepos, nil
}

// ArchMage checks that all architecture slice of a *HTTPRepo is filled right
func ArchMage(client *http.Client, repo *get.HTTPRepo) error {
	archsChan := make(chan string)
	// we need a dedicated goroutine to start the others, wait for them to finish
	// and signal back that we're done doing HTTP calls
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(architectures))

		// verify each arch page exists (possibly) in parallel
		for _, a := range architectures {
			go func(arch string) {
				defer wg.Done()

				if strings.Contains(repo.URL, arch) {
					archsChan <- arch
					return
				}

				finalUrl := repo.URL + arch + "/"
				exists, err := get.CheckWebPageExists(client, finalUrl)
				if err != nil {
					// TODO: verify if we need to actually return an error
					log.Printf("Got error calling HEAD %s: %v...\n", finalUrl, err)
				}
				if exists {
					archsChan <- arch
				}
			}(a)
		}

		wg.Wait()
		close(archsChan)
	}()

	for foundArch := range archsChan {
		repo.Archs = append(repo.Archs, foundArch)
	}

	if len(repo.Archs) == 0 {
		return fmt.Errorf("no available arch has been found for this repo: %s", repo.URL)
	}
	return nil
}

// GetRepo retrieves HTTP repositories data for all the products targets associated to an MU
func GetRepo(client *http.Client, mu string) (httpFormattedRepos []get.HTTPRepo, err error) {
	productsChunks, err := getProductsForMU(client, mu)
	if err != nil {
		return nil, fmt.Errorf("error retrieving products for MU %s: %v", mu, err)
	}
	fmt.Printf("%d product entries for mu %s\n", len(productsChunks), mu)

	reposChan := make(chan []get.HTTPRepo)
	errChan := make(chan error)
	// empty struct for 0 allocation: we need only to signal we're done, not pass data
	doneChan := make(chan struct{})

	// we need a dedicated goroutine to start the others, wait for them to finish
	// and signal back that we're done processing
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(productsChunks))

		// process each chunk (possibly) in parallel
		for _, productChunk := range productsChunks {
			go func(product, maint string) {
				defer wg.Done()

				repo, err := ProcWebChunk(client, product, maint)
				if err != nil {
					errChan <- err
				}
				reposChan <- repo

			}(productChunk, mu)
		}

		wg.Wait()
		close(reposChan)
		doneChan <- struct{}{}
	}()

	// keeps looping untill we're done processing all chunks or an error occurs
	for {
		select {
		case repo := <-reposChan:
			httpFormattedRepos = append(httpFormattedRepos, repo...)
		case err = <-errChan:
			return nil, err
		case <-doneChan:
			close(errChan)
			close(doneChan)
			return httpFormattedRepos, nil
		}
	}
}

// getProductsForMU parses a MU webpage attempting to retrieve a slice of available SUSE products
func getProductsForMU(client *http.Client, mu string) ([]string, error) {
	fmt.Println("GET", mu)
	resp, err := client.Get(mu)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	chunks := strings.Split(string(body), "\"")
	productsChunks := cleanWebChunks(chunks)

	return productsChunks, nil
}

// cleanWebChunks filters a slice of HTML elements and returns a slice containing only SUSE products
func cleanWebChunks(chunks []string) []string {
	products := []string{}
	regEx := regexp.MustCompile(`>(SUSE[^<]+\/)<`)

	for _, chunk := range chunks {
		matches := regEx.FindStringSubmatch(chunk)
		if matches != nil {
			products = append(products, matches[1])
		}
	}
	return products
}

func GetUpdatesAndChannels(usr, passwd string, justsearch bool) (updlist []Updates, err error) {
	client := maint.NewClient(usr, passwd)
	rrs, err := client.GetReleaseRequests("qam-manager", "new,review")
	if err != nil {
		return updlist, fmt.Errorf("error while getting response from obs: %v", err)
	}

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
			mu := fmt.Sprintf("%s%s/", maint.DownloadIbsLink, update.IncidentNumber)
			update.Repositories, err = GetRepo(client.HttpClient, mu)
			if err != nil {
				return updlist, fmt.Errorf("something went wrong in repo processing: %v", err)
			}
		}
		updlist = append(updlist, update)
	}
	return updlist, err
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
