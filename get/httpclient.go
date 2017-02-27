package get

import (
	"io"
	"net/http"
)

// Function maps a Reader to some other object
type Function func(io.ReadCloser) (result interface{}, err error)

// Apply applies a Function on data grabbed from an URL
func Apply(f Function, url string) (result interface{}, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	result, err = f(resp.Body)
	return
}

// ApplyStoring is like Apply and also saves a copy of processed data in a
// storage object
func ApplyStoring(f Function, url string, store *Storage, path string) (result interface{}, err error) {
	return Apply(func(r io.ReadCloser) (result interface{}, err error) {
		sr, err := store.NewStoringReader(path, r)
		if err != nil {
			return
		}
		defer sr.Close()

		result, err = f(sr)
		return
	}, url)
}
