package get

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// SCC defines the configuration to be used for downloading packages from SUSE Customer Center
type SCC struct {
	Username     string
	Password     string
	Repositories []SCCReposConfig
}

// SCCRepoConfig defines the configuration of SCC repos sharing the same architectures
type SCCReposConfig struct {
	Names []string
	Archs []string
}

// HTTP defines the configuration to be used for downloading packages from HTTP URLs
type HTTP struct {
	Repositories []HTTPReposConfig
}

// HTTPRepoConfig defines the configuration of HTTP repos sharing the same architectures
type HTTPReposConfig struct {
	URLs  []string
	Archs []string
}

// Repo represents the JSON entry for a repository as retuned by SCC API
type Repo struct {
	URL          string
	Name         string
	Description  string
	DistroTarget string `json:"distro_target"`
}

// maps a repo name to the available JSON entry from SCC API
type sccMap map[string]Repo

// SCCToHTTPConfigs returns HTTPS repos configurations (URLs and archs) for repos in SCC
func SCCToHTTPConfigs(baseURL string, username string, password string, sccConfigs []SCCReposConfig) ([]HTTPReposConfig, error) {
	sccEntries, err := getSCCEntries(baseURL, username, password)
	if err != nil {
		return nil, err
	}

	httpConfigs := []HTTPReposConfig{}
	for _, sccConfig := range sccConfigs {
		httpConfig := HTTPReposConfig{
			URLs:  []string{},
			Archs: sccConfig.Archs,
		}

		for _, name := range sccConfig.Names {
			repo, ok := sccEntries[name]
			if ok {
				for _, arch := range sccConfig.Archs {
					if strings.Contains(repo.Description, arch) {
						httpConfig.URLs = append(httpConfig.URLs, repo.URL)
						break
					}
				}
			}
		}

		if len(httpConfig.URLs) > 0 {
			httpConfigs = append(httpConfigs, httpConfig)
		}
	}
	return httpConfigs, nil
}

// getSCCEntries retrieves data from SCC API about the available repositories for an organizaion
func getSCCEntries(baseURL string, username, password string) (sccMap, error) {
	sccEntries := make(sccMap)
	token := base64.URLEncoding.EncodeToString([]byte(username + ":" + password))
	next := baseURL + "/connect/organizations/repositories"

	var page []byte
	var err error
	fmt.Println("Repos available in SCC follow:")
	for {
		page, next, err = downloadPaged(next, token)
		if err != nil {
			return nil, err
		}

		var repos []Repo
		err = json.Unmarshal(page, &repos)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			fmt.Printf("  %s: %s\n", repo.Name, repo.Description)
			sccEntries[repo.Name] = repo
		}

		if next == "" {
			break
		}
	}

	return sccEntries, nil
}

func downloadPaged(url string, token string) (page []byte, next string, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", token))
	req.Header.Add("Accept", "application/vnd.scc.suse.com.v4+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = &UnexpectedStatusCodeError{url, resp.StatusCode}
		return
	}

	page, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	re := regexp.MustCompile("<([^>]+)>; rel=\"next\"")
	matches := re.FindStringSubmatch(resp.Header["Link"][0])
	if matches != nil {
		next = matches[1]
	}

	return
}
