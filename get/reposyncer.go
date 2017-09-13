package get

import (
	"compress/gzip"
	"encoding/xml"
	"io"
	"log"

	"github.com/moio/minima/util"
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

const repomdPath = "repodata/repomd.xml"

type RepoSyncer struct {
	url     string
	archs   map[string]bool
	storage Storage
}

func NewRepoSyncer(url string, archs map[string]bool, storage Storage) *RepoSyncer {
	return &RepoSyncer{url, archs, storage}
}

// Stores a repo
func (r *RepoSyncer) StoreRepo() (err error) {
	files, err := r.processMetadata()
	if err != nil {
		return
	}

	log.Printf("Downloading %v packages...\n", len(files))
	for _, file := range files {
		log.Printf("...%v\n", file)
		err = r.downloadStoreApply(file, util.Nop)
		if err != nil {
			return
		}
	}
	return
}

// downloadStoreApply downloads a URL into a file, while applying a ReaderConsumer
func (r *RepoSyncer) downloadStoreApply(path string, f util.ReaderConsumer) error {
	return DownloadApply(r.url+"/"+path, util.Compose(r.storage.StoringMapper(path), f))
}

// processMetadata stores the repo metadata and returns a list of package file
// paths to download
func (r *RepoSyncer) processMetadata() (files []string, err error) {
	err = r.downloadStoreApply(repomdPath, func(reader io.ReadCloser) (err error) {
		decoder := xml.NewDecoder(reader)
		var repomd XmlRepomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		for i := 0; i < len(data); i++ {
			metadataPath := data[i].Location.Href
			if data[i].Type == "primary" {
				files, err = r.processPrimary(metadataPath)
			} else {
				err = r.downloadStoreApply(metadataPath, util.Nop)
			}
			if err != nil {
				return
			}
		}
		return
	})
	return
}

// processPrimary stores the primary XML metadata file and returns a list of
// package file paths to download
func (r *RepoSyncer) processPrimary(path string) (files []string, err error) {
	err = r.downloadStoreApply(path, func(reader io.ReadCloser) (err error) {
		gzReader, err := gzip.NewReader(reader)
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

		allArchs := len(r.archs) == 0
		for _, pack := range primary.Packages {
			if allArchs || pack.Arch == "noarch" || r.archs[pack.Arch] {
				if !r.storage.FileExists(pack.Location.Href) {
					log.Printf("...package '%v' not found, I will download it\n", pack.Location.Href)
					files = append(files, pack.Location.Href)
				} else {
					storageChecksum, err := r.storage.Checksum(pack.Location.Href, checksumTypeMap[pack.Checksum.Type])
					if err != nil {
						log.Printf("Checksum evaluation of the package '%v' returned the following error:\n", pack.Location.Href)
						log.Printf("Error message: %v\n", err)
						log.Println("...package skipped")
					} else if pack.Checksum.Checksum != storageChecksum {
						log.Printf("...package '%v' has a checksum error!!\n", pack.Location.Href)
						log.Printf("[repo vs local] = ['%v' VS '%v']\n", pack.Checksum.Checksum, storageChecksum)
						files = append(files, pack.Location.Href)
					} else {
						log.Printf("...package '%v' is up-to-date\n", pack.Location.Href)
					}
				}
			}
		}
		return
	})
	return
}
