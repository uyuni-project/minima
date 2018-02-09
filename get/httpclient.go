package get

import (
	"fmt"
	"io"
	"net/http"
)

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
