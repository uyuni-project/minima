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

// SCCRepoConfig defines the configuration of SCC repos sharing the same architectures
type SCCReposConfig struct {
	Names []string
	Archs []string
}

// HTTPRepoConfig defines the configuration of an HTTP repo
type HTTPRepoConfig struct {
	URL   string
	Archs []string
}

type Repo struct {
	URL          string
	Name         string
	Description  string
	DistroTarget string `json:"distro_target"`
}

// SCCToHTTPConfigs returns HTTPS repos configurations (URL and archs) for repos in SCC
func SCCToHTTPConfigs(baseURL string, username string, password string, sccConfigs []SCCReposConfig) ([]HTTPRepoConfig, error) {
	token := base64.URLEncoding.EncodeToString([]byte(username + ":" + password))
	httpConfigs := []HTTPRepoConfig{}

	var page []byte
	var err error
	next := baseURL + "/connect/organizations/repositories"

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
			config, ok := matchConfigs(repo.Name, repo.Description, repo.URL, sccConfigs)
			if ok {
				httpConfigs = append(httpConfigs, config)
			}
		}

		if next == "" {
			break
		}
	}

	return httpConfigs, nil
}

// matchConfigs attempts to match the given name and description to one of the given SCCReposConfig
// and build a HTTRepoConfig for it.
//
// Returns a HTTPRepoConfig and a bool indicating whether the match was successfull or not.
func matchConfigs(name, description, url string, repoConfigs []SCCReposConfig) (HTTPRepoConfig, bool) {
	httpConfig := HTTPRepoConfig{
		Archs: []string{},
	}

	for _, config := range repoConfigs {
		for _, repoName := range config.Names {
			if strings.Contains(name, repoName) {
				for _, arch := range config.Archs {
					if strings.Contains(description, arch) {
						httpConfig.Archs = append(httpConfig.Archs, arch)
					}
				}
				if len(httpConfig.Archs) > 0 {
					httpConfig.URL = url
					return httpConfig, true
				}
			}
		}
	}
	return httpConfig, false
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
