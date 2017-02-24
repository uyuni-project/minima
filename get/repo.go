package get

import (
	"encoding/xml"
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

// Get downloads one repo and returns its metadata
func Get(url string) (repo map[string]string, err error) {
	resp, err := http.Get(url + "/repodata/repomd.xml")
	if err != nil {
		return
	}

	defer resp.Body.Close()
	decoder := xml.NewDecoder(resp.Body)
	var repomd Repomd
	err = decoder.Decode(&repomd)

	data := repomd.Data
	repo = make(map[string]string)
	for i := 0; i < len(data); i++ {
		repo[data[i].Type] = data[i].Location.Href
	}

	return
}
