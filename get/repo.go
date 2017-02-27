package get

import (
	"encoding/xml"
	"io"
	"net/http"
)

type Repomd struct {
	Data []Data `xml:"data"`
}

type Data struct {
	Type     string   `xml:"type,attr"`
	Location Location `xml:"location"`
}

type Location struct {
	Href string `xml:"href,attr"`
}

const repomdLocation = "/repodata/repomd.xml"

// Function maps a Reader to some other object
type Function func(io.ReadCloser) (result interface{}, err error)

// Do runs a Function on data grabbed from an URL
func Do(url string, f Function) (result interface{}, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}

	result, err = f(resp.Body)
	return
}

// StoringDo is like Do but also archives a copy of the data to a Storage object
func StoringDo(url string, storage *Storage, f Function) (result interface{}, err error) {
	return Do(url, func(reader io.ReadCloser) (result interface{}, err error) {
		storingReader, err := storage.NewStoringReader(repomdLocation, reader)
		if err != nil {
			return
		}
		defer storingReader.Close()

		result, err = f(storingReader)
		return
	})
}

// Stores a repo metadata files
func StoreMetadata(url string, storage *Storage) (result map[string]string, err error) {
	rawResult, err := StoringDo(url+repomdLocation, storage, func(r io.ReadCloser) (result interface{}, err error) {
		decoder := xml.NewDecoder(r)
		var repomd Repomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		metadata := make(map[string]string)
		for i := 0; i < len(data); i++ {
			metadata[data[i].Type] = data[i].Location.Href
		}
		result = metadata
		return
	})
	if err != nil {
		return
	}

	result = rawResult.(map[string]string)
	return
}
