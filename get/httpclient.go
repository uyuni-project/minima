package get

import (
	"fmt"
	"io"
	"net/http"
)

// HTTPRepo defines the configuration of an HTTP repo
type HTTPRepo struct {
	URL   string
	Archs []string
}

// UnexpectedStatusCodeError signals a successful request that resulted in an unexpected status code
type UnexpectedStatusCodeError struct {
	URL        string
	StatusCode int
}

func (e UnexpectedStatusCodeError) Error() string {
	return fmt.Sprintf("Got unexpected status code from %s, %d", e.URL, e.StatusCode)
}

// ReadURL returns a Reader for bytes from an http URL
func ReadURL(url string) (r io.ReadCloser, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	if response.StatusCode != 200 {
		err = &UnexpectedStatusCodeError{url, response.StatusCode}
		return
	}

	r = response.Body

	return
}

func CheckWebPageExists(client *http.Client, repoURL string) (bool, error) {
	resp, err := client.Head(repoURL)
	if err != nil {
		return false, err
	}

	return resp.Status == "200 OK", nil
}
