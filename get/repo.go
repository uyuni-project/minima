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
func StoreMetadata(url string, storage *Storage) (err error) {
	_, err = ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
		decoder := xml.NewDecoder(r)
		var repomd Repomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		for i := 0; i < len(data); i++ {
      href := data[i].Location.Href
			_, err = ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
        return
      }, url + "/" + href, storage, href)
      if err != nil {
        return
      }
		}
		return
	}, url + repomdLocation, storage, repomdLocation)
  return
}
