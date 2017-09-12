package get

import (
	"net/http"

	"github.com/moio/minima/util"
)

// DownloadApply downloads bytes from an URL and applies a ReaderConsumer function to the result
func DownloadApply(url string, f util.ReaderConsumer) (err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	body := response.Body

	err = f(body)
	return
}
