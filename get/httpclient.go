package get

import (
	"io"
	"net/http"

	"github.com/moio/minima/util"
)

// Apply applies a ReaderFunction on data grabbed from an URL
func Apply(f util.ReaderFunction, url string) (result interface{}, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	result, err = f(resp.Body)
	return
}

// ApplyStoring is like Apply and also saves a copy of processed data in a
// storage object
func ApplyStoring(f util.ReaderFunction, url string, mapper util.ReaderMapper) (result interface{}, err error) {
	return Apply(func(r io.ReadCloser) (result interface{}, err error) {
		mappedR, err := mapper(r)
		if err != nil {
			return
		}
		defer mappedR.Close()

		result, err = f(mappedR)
		return
	}, url)
}

// Store saves data from an url in a storage object
func Store(url string, mapper util.ReaderMapper) (err error) {
	_, err = ApplyStoring(util.NopReaderFunction, url, mapper)
	return
}
