package get

import (
	"compress/gzip"
	"crypto"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"

	"github.com/moio/minima/util"
)

// common

// XMLLocation maps a <location> tag in repodata/repomd.xml or repodata/<ID>-primary.xml.gz
type XMLLocation struct {
	Href string `xml:"href,attr"`
}

// repodata/repomd.xml

// XMLRepomd maps a <repomd> tag in repodata/repomd.xml
type XMLRepomd struct {
	Data []XMLData `xml:"data"`
}

// XMLData maps a <data> tag in repodata/repomd.xml
type XMLData struct {
	Type     string      `xml:"type,attr"`
	Location XMLLocation `xml:"location"`
	Checksum XMLChecksum `xml:"checksum"`
}

// repodata/<ID>-primary.xml.gz

// XMLMetaData maps a <metadata> tag in repodata/<ID>-primary.xml.gz
type XMLMetaData struct {
	Packages []XMLPackage `xml:"package"`
}

// XMLPackage maps a <package> tag in repodata/<ID>-primary.xml.gz
type XMLPackage struct {
	Arch     string      `xml:"arch"`
	Location XMLLocation `xml:"location"`
	Checksum XMLChecksum `xml:"checksum"`
}

// XMLChecksum maps a <checksum> tag in repodata/<ID>-primary.xml.gz
type XMLChecksum struct {
	Type     string `xml:"type,attr"`
	Checksum string `xml:",cdata"`
}

var hashMap = map[string]crypto.Hash{
	"sha":    crypto.SHA1,
	"sha1":   crypto.SHA1,
	"sha256": crypto.SHA256,
}

const repomdPath = "repodata/repomd.xml"

// Syncer syncs repos from an HTTP source to a Storage
type Syncer struct {
	// URL of the repo this syncer syncs
	URL     url.URL
	archs   map[string]bool
	storage Storage
}

// NewSyncer creates a new Syncer
func NewSyncer(url url.URL, archs map[string]bool, storage Storage) *Syncer {
	return &Syncer{url, archs, storage}
}

// StoreRepo stores an HTTP repo in a Storage, automatically retrying in case of recoverable errors
func (r *Syncer) StoreRepo() (err error) {
	checksumMap := r.readChecksumMap()
	for i := 0; i < 20; i++ {
		err = r.storeRepo(checksumMap)
		if err == nil {
			return
		}

		uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
		if unexpectedStatusCode {
			if uerr.StatusCode == 404 {
				log.Printf("Got 404, presumably temporarily, retrying...\n")
			} else {
				return err
			}
		}

		_, checksumError := err.(*util.ChecksumError)
		if checksumError {
			log.Printf(err.Error())
			log.Printf("Checksum did not match, presumably the repo was published while syncing, retrying...\n")
		} else {
			return err
		}
	}

	log.Printf("Too many temporary errors, aborting...\n")
	return err
}

// StoreRepo stores an HTTP repo in a Storage
func (r *Syncer) storeRepo(checksumMap map[string]XMLChecksum) (err error) {
	packagesToDownload, packagesToRecycle, err := r.processMetadata(checksumMap)
	if err != nil {
		return
	}

	downloadCount := len(packagesToDownload)
	log.Printf("Downloading %v packages...\n", downloadCount)
	for i, pack := range packagesToDownload {
		description := fmt.Sprintf("(%v/%v) %v", i+1, downloadCount, path.Base(pack.Location.Href))
		err = r.downloadStoreApply(pack.Location.Href, pack.Checksum.Checksum, description, hashMap[pack.Checksum.Type], util.Nop)
		if err != nil {
			return err
		}
	}

	recycleCount := len(packagesToRecycle)
	log.Printf("Recycling %v packages...\n", recycleCount)
	for _, pack := range packagesToRecycle {
		err = r.storage.Recycle(pack.Location.Href)
		if err != nil {
			return
		}
	}

	log.Printf("Committing changes...\n")
	err = r.storage.Commit()
	if err != nil {
		return
	}
	return
}

// downloadStore downloads a repo-relative path into a file
func (r *Syncer) downloadStore(path string, description string) error {
	return r.downloadStoreApply(path, "", description, 0, util.Nop)
}

// downloadStoreApply downloads a repo-relative path into a file, while applying a ReaderConsumer
func (r *Syncer) downloadStoreApply(relativePath string, checksum string, description string, hash crypto.Hash, f util.ReaderConsumer) error {
	log.Printf("Downloading %v...", description)
	url := r.URL
	url.Path = path.Join(r.URL.Path, relativePath)
	body, err := ReadURL(url.String())
	if err != nil {
		return err
	}

	return util.Compose(r.storage.StoringMapper(relativePath, checksum, hash), f)(body)
}

// processMetadata stores the repo metadata and returns a list of package file
// paths to download
func (r *Syncer) processMetadata(checksumMap map[string]XMLChecksum) (packagesToDownload []XMLPackage, packagesToRecycle []XMLPackage, err error) {
	err = r.downloadStoreApply(repomdPath, "", path.Base(repomdPath), 0, func(reader io.ReadCloser) (err error) {
		decoder := xml.NewDecoder(reader)
		var repomd XMLRepomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		for i := 0; i < len(data); i++ {
			log.Printf(data[i].Location.Href)
			metadataLocation := data[i].Location.Href
			metadataChecksum := data[i].Checksum
			decision := r.decide(metadataLocation, metadataChecksum, checksumMap)
			switch decision {
			case Download:
				log.Printf("...downloading")
				err = r.downloadStoreApply(metadataLocation, metadataChecksum.Checksum, path.Base(metadataLocation), hashMap[metadataChecksum.Type], util.Nop)
				if err != nil {
					return
				}
			case Recycle:
				log.Printf("...recycling")
				r.storage.Recycle(metadataLocation)
			}

			if data[i].Type == "primary" {
				packagesToDownload, packagesToRecycle, err = r.processPrimary(metadataLocation, checksumMap)
			}
		}
		return
	})
	if err != nil {
		return
	}

	ascPath := repomdPath + ".asc"
	err = r.downloadStore(ascPath, path.Base(ascPath))
	if err != nil {
		uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
		if unexpectedStatusCode && uerr.StatusCode == 404 {
			log.Printf("Got 404, ignoring...")
		} else {
			return
		}
	}

	keyPath := repomdPath + ".key"
	err = r.downloadStore(keyPath, path.Base(keyPath))
	if err != nil {
		uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
		if unexpectedStatusCode && uerr.StatusCode == 404 {
			log.Printf("Got 404, ignoring...")
			err = nil
		} else {
			return
		}
	}

	return
}

func (r *Syncer) readMetaData(reader io.Reader) (primary XMLMetaData, err error) {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return
	}
	defer gzReader.Close()

	decoder := xml.NewDecoder(gzReader)
	err = decoder.Decode(&primary)

	return
}

func (r *Syncer) readChecksumMap() (checksumMap map[string]XMLChecksum) {
	checksumMap = make(map[string]XMLChecksum)
	repomdReader, err := r.storage.NewReader(repomdPath, Permanent)
	if err != nil {
		if err == ErrFileNotFound {
			log.Println("First-time sync started")
		} else {
			log.Println(err.Error())
			log.Println("Error while reading previously-downloaded metadata. Starting sync from scratch")
		}
		return
	}
	defer repomdReader.Close()

	decoder := xml.NewDecoder(repomdReader)
	var repomd XMLRepomd
	err = decoder.Decode(&repomd)
	if err != nil {
		log.Println(err.Error())
		log.Println("Error while parsing previously-downloaded metadata. Starting sync from scratch")
		return
	}

	data := repomd.Data
	for i := 0; i < len(data); i++ {
		dataHref := data[i].Location.Href
		dataChecksum := data[i].Checksum
		checksumMap[dataHref] = dataChecksum
		if data[i].Type == "primary" {
			primaryReader, err := r.storage.NewReader(dataHref, Permanent)
			if err != nil {
				return
			}
			primary, err := r.readMetaData(primaryReader)
			if err != nil {
				return
			}
			for _, pack := range primary.Packages {
				checksumMap[pack.Location.Href] = pack.Checksum
			}
		}
	}
	return
}

// processPrimary stores the primary XML metadata file and returns a list of
// package file paths to download
func (r *Syncer) processPrimary(path string, checksumMap map[string]XMLChecksum) (packagesToDownload []XMLPackage, packagesToRecycle []XMLPackage, err error) {
	reader, err := r.storage.NewReader(path, Temporary)
	if err != nil {
		return
	}
	primary, err := r.readMetaData(reader)
	if err != nil {
		return
	}

	allArchs := len(r.archs) == 0
	for _, pack := range primary.Packages {
		if allArchs || pack.Arch == "noarch" || r.archs[pack.Arch] {
			decision := r.decide(pack.Location.Href, pack.Checksum, checksumMap)
			switch decision {
			case Download:
				packagesToDownload = append(packagesToDownload, pack)
			case Recycle:
				packagesToRecycle = append(packagesToRecycle, pack)
			}
		}
	}
	return
}

// Decision encodes what to do with a file
type Decision int

const (
	// Download means the Syncer will download a file
	Download Decision = iota
	// Recycle means the Syncer will copy an existing file without downloading
	Recycle
	// Skip means the Syncer detected an already-existing file and has nothing to do
	Skip
)

func (r *Syncer) decide(location string, checksum XMLChecksum, checksumMap map[string]XMLChecksum) Decision {
	previousChecksum, foundInPermanentLocation := checksumMap[location]
	if !foundInPermanentLocation || previousChecksum.Type != checksum.Type || previousChecksum.Checksum != checksum.Checksum {
		reader, err := r.storage.NewReader(location, Temporary)
		if err != nil {
			return Download
		}
		defer reader.Close()
		readChecksum, err := util.Checksum(reader, hashMap[checksum.Type])
		if err != nil || readChecksum != checksum.Checksum {
			return Download
		}
		return Skip
	}
	return Recycle
}
