package get

import (
	"io/ioutil"
	"net/http"
)

// Get downloads one repo and returns its metadata
func Get(url string) (body string, err error) {
	resp, err := http.Get(url + "/repodata/repomd.xml")
	if err != nil {
		return
	}

	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	body = string(bytes)
	return
}
