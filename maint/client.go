// Lib's Co-Author: Felix Gerling, fgerling@suse.com, https://github.com/fgerling
package maint

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/storage"
)

// package scoped array of all possible available archs to check for a repo
var architectures = [...]string{"x86_64", "i586", "i686", "aarch64", "aarch64_ilp32", "ppc64le", "s390x", "src"}

// package scoped Thread-safe Map used as cache to check the existence of repositories
var register sync.Map

type BuildService string

const (
	IBS BuildService = "ibs"
	OBS BuildService = "obs"

	downloadOBSLink = "http://download.opensuse.org/repositories/openSUSE:/Maintenance:/"
	downloadIBSLink = "http://download.suse.de/ibs/SUSE:/Maintenance:/"
	obsAPI          = "api.opensuse.org"
	ibsAPI          = "api.suse.de"
)

type BuildServiceCredentials struct {
	API      BuildService
	Username string
	Password string
}

type BuildServiceClient struct {
	downloadLink string
	baseURL      *url.URL
	username     string
	password     string
	httpClient   *http.Client
}

var ErrUnsupportedAPI = errors.New("unsupported build service API (must be 'obs' or 'ibs')")

func NewBuildServiceClient(buildService BuildService, username string, password string) (*BuildServiceClient, error) {
	client := &BuildServiceClient{
		username:   username,
		password:   password,
		httpClient: &http.Client{},
	}

	switch buildService {
	case IBS:
		client.downloadLink = downloadIBSLink
		client.baseURL = &url.URL{Host: ibsAPI, Scheme: "https"}
	case OBS:
		client.downloadLink = downloadOBSLink
		client.baseURL = &url.URL{Host: obsAPI, Scheme: "https"}
	default:
		return nil, ErrUnsupportedAPI
	}

	return client, nil
}

func (c *BuildServiceClient) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.baseURL.ResolveReference(rel)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := xml.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.username, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
	}

	req.Header.Set("Accept", "application/xml")
	return req, nil
}

func (c *BuildServiceClient) GetReleaseRequests(group string, states string) ([]ReleaseRequest, error) {
	req, err := c.newRequest("GET", "/request", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("view", "collection")
	q.Add("group", group)
	q.Add("states", states)
	req.URL.RawQuery = q.Encode()

	var collection Collection
	err = c.do(req, &collection)
	return collection.ReleaseRequests, err
}

func (c *BuildServiceClient) GetPatchinfo(rr ReleaseRequest) (*Patchinfo, error) {
	project := rr.Actions[0].Source.Project
	patchinfo_url := fmt.Sprintf("/source/%v/patchinfo/_patchinfo", project)

	req, err := c.newRequest("GET", patchinfo_url, nil)
	if err != nil {
		return nil, err
	}

	var patchinfo Patchinfo
	err = c.do(req, &patchinfo)
	return &patchinfo, err
}

func (c *BuildServiceClient) GetUpdatesAndChannels(group, states string, justsearch bool) ([]Updates, error) {
	rrs, err := c.GetReleaseRequests(group, states)
	if err != nil {
		return nil, fmt.Errorf("error while getting response from obs: %v", err)
	}

	updates := []Updates{}
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
			mu := fmt.Sprintf("%s%s/", c.downloadLink, update.IncidentNumber)
			update.Repositories, err = c.GetRepo(mu)
			if err != nil {
				return nil, fmt.Errorf("something went wrong in repo processing: %v", err)
			}
		}
		updates = append(updates, update)
	}

	return updates, err
}

func (c *BuildServiceClient) RemoveOldChannels(config storage.StorageConfig, updates []Updates) error {
	incidentNumbers := toIncidentNumberSet(updates)

	switch config.Type {
	case "file":
		var muChannelList []string

		downloadURL, err := url.Parse(c.downloadLink)
		if err != nil {
			return err
		}

		err = filepath.Walk(filepath.Join(config.Path, downloadURL.Path[1:]), func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				muChannelList = append(muChannelList, path)
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, elem := range muChannelList {
			if regexp.MustCompile(`/\d{5,6}/`).FindString(elem) != "" {
				_, exists := incidentNumbers[strings.Replace(regexp.MustCompile(`/\d{5,6}/`).FindString(elem), "/", "", 10)]
				if !exists {
					log.Printf("removing: %s...\n", elem)
					err := os.RemoveAll(elem)
					if err != nil {
						return err
					}
				}
			}
		}
	default:
		return fmt.Errorf("unsupported storage type for cleanup")
	}
	return nil
}

// GetRepo retrieves HTTP repositories data for all the products targets associated to an MU
func (c *BuildServiceClient) GetRepo(incidentNumber string) (httpFormattedRepos []get.HTTPRepo, err error) {
	mu := fmt.Sprintf("%s%s/", c.downloadLink, incidentNumber)

	productsChunks, err := c.getProductsForMU(mu)
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

				repo, err := c.procWebChunk(product, maint)
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

	// keeps looping until we're done processing all chunks or an error occurs
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

// ProcWebChunk retrieves repositories data for a product target in a MU
func (c *BuildServiceClient) procWebChunk(product, maint string) ([]get.HTTPRepo, error) {
	httpFormattedRepos := []get.HTTPRepo{}
	repo := get.HTTPRepo{
		Archs: []string{},
	}
	repoUrl := maint + product

	_, ok := register.Load(repoUrl)
	if !ok {
		exists, err := get.CheckWebPageExists(c.httpClient, repoUrl)
		if err != nil {
			return nil, err
		}
		register.Store(repoUrl, exists)

		if exists {
			repo.URL = repoUrl
			if err := c.archMage(&repo); err != nil {
				return nil, err
			}
			fmt.Println(repo)
			httpFormattedRepos = append(httpFormattedRepos, repo)
		}
	}
	return httpFormattedRepos, nil
}

// archMage checks that all architecture slice of a *HTTPRepo is filled right
func (c *BuildServiceClient) archMage(repo *get.HTTPRepo) error {
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
				exists, err := get.CheckWebPageExists(c.httpClient, finalUrl)
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

// getProductsForMU parses a MU webpage attempting to retrieve a slice of available SUSE products
func (c *BuildServiceClient) getProductsForMU(mu string) ([]string, error) {
	fmt.Println("GET", mu)
	resp, err := c.httpClient.Get(mu)
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

func (c *BuildServiceClient) do(req *http.Request, v interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("got status code: %v for %q", resp.StatusCode, req.URL)
	}
	defer resp.Body.Close()

	err = xml.NewDecoder(resp.Body).Decode(v)
	return err
}
