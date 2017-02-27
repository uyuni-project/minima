package get

import (
	"compress/gzip"
	"encoding/xml"
	"io"
	"log"
)

// common

// Location maps a <location> tag in repodata/repomd.xml or repodata/<ID>-primary.xml.gz
type Location struct {
	Href string `xml:"href,attr"`
}

// repodata/repomd.xml

// Repomd maps a <repomd> tag in repodata/repomd.xml
type Repomd struct {
	Data []Data `xml:"data"`
}

// Data maps a <data> tag in repodata/repomd.xml
type Data struct {
	Type     string   `xml:"type,attr"`
	Location Location `xml:"location"`
}

// repodata/<ID>-primary.xml.gz

// Metadata maps a <metadata> tag in repodata/<ID>-primary.xml.gz
type Metadata struct {
	Packages []Package `xml:"package"`
}

// Package maps a <package> tag in repodata/<ID>-primary.xml.gz
type Package struct {
	Arch     string   `xml:"arch"`
	Location Location `xml:"location"`
}

const repomdPath = "/repodata/repomd.xml"

// Stores a repo
func StoreRepo(url string, storage *Storage, archs map[string]bool) (err error) {
	packagePaths, err := processMetadata(url, storage, archs)
	if err != nil {
		return
	}

	log.Printf("Downloading %v packages...\n", len(packagePaths))
	for i := 0; i < len(packagePaths); i++ {
		packagePath := packagePaths[i]
		log.Printf("...%v\n", packagePath)
		err = Store(url+"/"+packagePath, storage, packagePath)
		if err != nil {
			return
		}
	}
	return
}

// processMetadata stores the repo metadata and returns a list of package file
// paths to download
func processMetadata(url string, storage *Storage, archs map[string]bool) (packagePaths []string, err error) {
	_, err = ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
		decoder := xml.NewDecoder(r)
		var repomd Repomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		for i := 0; i < len(data); i++ {
			metadataPath := data[i].Location.Href
			metadataUrl := url + "/" + metadataPath
			if data[i].Type == "primary" {
				packagePaths, err = processPrimary(metadataUrl, storage, metadataPath, archs)
			} else {
				err = Store(metadataUrl, storage, metadataPath)
			}
			if err != nil {
				return
			}
		}
		return
	}, url+repomdPath, storage, repomdPath)
	return
}

// processPrimary stores the primary XML metadata file and returns a list of
// package file paths to download
func processPrimary(url string, storage *Storage, path string, archs map[string]bool) (packagePaths []string, err error) {
	_, err = ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return
		}
		defer gzReader.Close()

		decoder := xml.NewDecoder(gzReader)
		var primary Metadata
		err = decoder.Decode(&primary)
		if err != nil {
			return
		}

		archCount := len(archs)
		for _, pack := range primary.Packages {
			if archCount == 0 || archs[pack.Arch] {
				packagePaths = append(packagePaths, pack.Location.Href)
			}
		}
		return
	}, url, storage, path)
	return
}
