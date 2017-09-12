package get

import (
	"io"
	"net/http"
)

// ReaderF maps a Reader to some other object
type ReaderF func(io.ReadCloser) (result interface{}, err error)

// ReaderMapper maps a Reader to another Reader
type ReaderMapper func(io.ReadCloser) (result io.ReadCloser, err error)

// nop maps a Reader to nothing
func nop(r io.ReadCloser) (result interface{}, err error) {
	return
}

// Apply applies a ReaderF on data grabbed from an URL
func Apply(f ReaderF, url string) (result interface{}, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	result, err = f(resp.Body)
	return
}

// ApplyStoring is like Apply and also saves a copy of processed data in a
// storage object
func ApplyStoring(f ReaderF, url string, mapper ReaderMapper) (result interface{}, err error) {
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
func Store(url string, mapper ReaderMapper) (err error) {
	_, err = ApplyStoring(nop, url, mapper)
	return
}
