package get

import (
	"net/http"

	"github.com/moio/minima/util"
)

// GetApply downloads bytes from an URL via GET and applies a ReaderFunction to the result
func GetApply(url string, f util.ReaderFunction) (result interface{}, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	body := response.Body

	result, err = f(body)
	return
}
