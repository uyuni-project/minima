package get

import (
	"fmt"
	"io"
	"net/http"
)

// ReadURL returns a Reader for bytes from an http URL
func ReadURL(url string) (r io.ReadCloser, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	if response.StatusCode != 200 {
		err = fmt.Errorf("Got unexpected status code from %s, %d", url, response.StatusCode)
		return
	}

	r = response.Body

	return
}
