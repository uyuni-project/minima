package get

import (
	"net/http"

	"github.com/moio/minima/util"
)

// ApplyStoring is like apply and also saves a copy of processed data in a
// storage object
func ApplyStoring(f util.ReaderFunction, url string, mapper util.ReaderMapper) (result interface{}, err error) {
	response, err := http.Get(url)
	if err != nil {
		return
	}

	reader := response.Body
	mappedReader, err := mapper(reader)
	if err != nil {
		return
	}
	defer mappedReader.Close()

	result, err = f(mappedReader)
	return
}

// Store saves data from an url in a storage object
func Store(url string, mapper util.ReaderMapper) (err error) {
	_, err = ApplyStoring(util.NopReaderFunction, url, mapper)
	return
}
