package get

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

// SCCURLs returns URLs for repos in SCC
func SCCURLs(baseURL string, username string, password string, nameFilters []string, descriptionFilters []string) (urls []string, err error) {
	urls = []string{}

	token := base64.URLEncoding.EncodeToString([]byte(username + ":" + password))

	fmt.Println("Repos available in SCC follow:")
	next := baseURL + "/connect/organizations/repositories"
	for {
		var page []byte
		page, next, err = _downloadPaged(next, token)
		if err != nil {
			return nil, err
		}

		type Repo struct {
			URL          string
			Name         string
			Description  string
			DistroTarget string `json:"distro_target"`
		}

		var repos []Repo
		err := json.Unmarshal(page, &repos)
		if err != nil {
			return nil, err
		}

		for _, repo := range repos {
			fmt.Printf("  %s: %s\n", repo.Name, repo.Description)
			if _matches(repo.Name, repo.Description, nameFilters, descriptionFilters) {
				urls = append(urls, repo.URL)
			}
		}

		if next == "" {
			break
		}
	}

	return
}

func _matches(name string, description string, nameFilters []string, descriptionFilters []string) bool {
	for _, nameFilter := range nameFilters {
		if strings.Contains(name, nameFilter) {
			for _, descriptionFilter := range descriptionFilters {
				if strings.Contains(description, descriptionFilter) {
					return true
				}
			}
		}
	}
	return false
}

func _downloadPaged(url string, token string) (page []byte, next string, err error) {
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

	page, err = ioutil.ReadAll(resp.Body)
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
