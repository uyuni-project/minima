package get

import (
	"encoding/xml"
	"io"
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

// Stores a repo metadata files
func StoreMetadata(url string, storage *Storage) (result map[string]string, err error) {
	rawResult, err := ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
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
	}, url + repomdLocation, storage, repomdLocation)
	if err != nil {
		return
	}

	result = rawResult.(map[string]string)
	return
}
