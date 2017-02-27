package get

import (
	"compress/gzip"
	"encoding/xml"
	"io"
	"log"
)

// common

// XmlLocation maps a <location> tag in repodata/repomd.xml or repodata/<ID>-primary.xml.gz
type XmlLocation struct {
	Href string `xml:"href,attr"`
}

// repodata/repomd.xml

// XmlRepomd maps a <repomd> tag in repodata/repomd.xml
type XmlRepomd struct {
	Data []XmlData `xml:"data"`
}

// XmlData maps a <data> tag in repodata/repomd.xml
type XmlData struct {
	Type     string      `xml:"type,attr"`
	Location XmlLocation `xml:"location"`
}

// repodata/<ID>-primary.xml.gz

// XmlMetadata maps a <metadata> tag in repodata/<ID>-primary.xml.gz
type XmlMetadata struct {
	Packages []XmlPackage `xml:"package"`
}

// XmlPackage maps a <package> tag in repodata/<ID>-primary.xml.gz
type XmlPackage struct {
	Arch     string      `xml:"arch"`
	Location XmlLocation `xml:"location"`
	Checksum XmlChecksum `xml:"checksum"`
}

// XmlChecksum maps a <checksum> tag in repodata/<ID>-primary.xml.gz
type XmlChecksum struct {
	Type     string `xml:"type,attr"`
	Checksum string `xml:",cdata"`
}

type ChecksumType int

const (
	SHA1 ChecksumType = iota
	SHA256
)

type PackageFile struct {
	Path         string
	Checksum     string
	ChecksumType ChecksumType
}

var checksumTypeMap = map[string]ChecksumType{
	"sha":    SHA1,
	"sha1":   SHA1,
	"sha256": SHA256,
}

const repomdPath = "/repodata/repomd.xml"

// Stores a repo
func StoreRepo(url string, storage *Storage, archs map[string]bool) (err error) {
	files, err := processMetadata(url, storage, archs)
	if err != nil {
		return
	}

	log.Printf("Downloading %v packages...\n", len(files))
	for _, file := range files {
		log.Printf("...%v\n", file.Path)
		err = Store(url+"/"+file.Path, storage, file.Path)
		if err != nil {
			return
		}
	}
	return
}

// processMetadata stores the repo metadata and returns a list of package file
// paths to download
func processMetadata(url string, storage *Storage, archs map[string]bool) (files []PackageFile, err error) {
	_, err = ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
		decoder := xml.NewDecoder(r)
		var repomd XmlRepomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		for i := 0; i < len(data); i++ {
			metadataPath := data[i].Location.Href
			metadataUrl := url + "/" + metadataPath
			if data[i].Type == "primary" {
				files, err = processPrimary(metadataUrl, storage, metadataPath, archs)
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
func processPrimary(url string, storage *Storage, path string, archs map[string]bool) (files []PackageFile, err error) {
	_, err = ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return
		}
		defer gzReader.Close()

		decoder := xml.NewDecoder(gzReader)
		var primary XmlMetadata
		err = decoder.Decode(&primary)
		if err != nil {
			return
		}

		archCount := len(archs)
		for _, pack := range primary.Packages {
			if archCount == 0 || archs[pack.Arch] {
				log.Println(pack.Checksum)
				files = append(files, PackageFile{pack.Location.Href, pack.Checksum.Checksum, checksumTypeMap[pack.Checksum.Type]})
			}
		}
		return
	}, url, storage, path)
	return
}
