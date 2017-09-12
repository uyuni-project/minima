package get

import (
	"net/http"

	"github.com/moio/minima/util"
)

// GetApply downloads bytes from an URL via GET and applies a ReaderConsumer to the result
func GetApply(url string, f util.ReaderConsumer) (err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	body := response.Body

	err = f(body)
	return
}
