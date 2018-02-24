package get

import (
	"compress/gzip"
	"crypto"
	"encoding/xml"
	"io"
	"log"

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
	Url     string
	archs   map[string]bool
	storage Storage
}

// NewSyncer creates a new Syncer
func NewSyncer(url string, archs map[string]bool, storage Storage) *Syncer {
	return &Syncer{url, archs, storage}
}

// StoreRepo stores an HTTP repo in a Storage, automatically retrying in case of recoverable errors
func (r *Syncer) StoreRepo() (err error) {
	checksumMap := r.readChecksumMap()
	for i := 0; i < 10; i++ {
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
	for _, pack := range packagesToDownload {
		err = r.downloadStoreApply(pack.Location.Href, pack.Checksum.Checksum, hashMap[pack.Checksum.Type], util.Nop)
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
func (r *Syncer) downloadStore(path string) error {
	return r.downloadStoreApply(path, "", 0, util.Nop)
}

// downloadStoreApply downloads a repo-relative path into a file, while applying a ReaderConsumer
func (r *Syncer) downloadStoreApply(path string, checksum string, hash crypto.Hash, f util.ReaderConsumer) error {
	log.Printf("Downloading %v...", path)
	body, err := ReadURL(r.Url + "/" + path)
	if err != nil {
		return err
	}

	return util.Compose(r.storage.StoringMapper(path, checksum, hash), f)(body)
}

// processMetadata stores the repo metadata and returns a list of package file
// paths to download
func (r *Syncer) processMetadata(checksumMap map[string]XMLChecksum) (packagesToDownload []XMLPackage, packagesToRecycle []XMLPackage, err error) {
	err = r.downloadStoreApply(repomdPath, "", 0, func(reader io.ReadCloser) (err error) {
		decoder := xml.NewDecoder(reader)
		var repomd XMLRepomd
		err = decoder.Decode(&repomd)
		if err != nil {
			return
		}

		data := repomd.Data
		for i := 0; i < len(data); i++ {
			metadataPath := data[i].Location.Href
			if data[i].Type == "primary" {
				packagesToDownload, packagesToRecycle, err = r.processPrimary(metadataPath, checksumMap)
			} else {
				err = r.downloadStore(metadataPath)
			}
			if err != nil {
				return
			}
		}
		return
	})
	if err != nil {
		return
	}

	err = r.downloadStore(repomdPath + ".asc")
	if err != nil {
		uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
		if unexpectedStatusCode && uerr.StatusCode == 404 {
			log.Printf("Got 404, ignoring...")
		} else {
			return
		}
	}

	err = r.downloadStore(repomdPath + ".key")
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
		metadataPath := data[i].Location.Href
		if data[i].Type == "primary" {
			primaryReader, err := r.storage.NewReader(metadataPath, Permanent)
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
	err = r.downloadStoreApply(path, "", 0, func(reader io.ReadCloser) (err error) {
		primary, err := r.readMetaData(reader)
		if err != nil {
			return
		}

		allArchs := len(r.archs) == 0
		for _, pack := range primary.Packages {
			if allArchs || pack.Arch == "noarch" || r.archs[pack.Arch] {
				previousChecksum, foundInPermanentLocation := checksumMap[pack.Location.Href]
				if !foundInPermanentLocation || previousChecksum.Type != pack.Checksum.Type || previousChecksum.Checksum != pack.Checksum.Checksum {
					reader, err := r.storage.NewReader(pack.Location.Href, Temporary)
					if err != nil {
						log.Printf("...package '%v' not found or not recyclable, will be downloaded\n", pack.Location.Href)
						packagesToDownload = append(packagesToDownload, pack)
					} else {
						checksum, err := util.Checksum(reader, hashMap[pack.Checksum.Type])
						if err != nil || checksum != pack.Checksum.Checksum {
							log.Printf("...package '%v' found in partially-downloaded repo, not recyclable, will be re-downloaded\n", pack.Location.Href)
							packagesToDownload = append(packagesToDownload, pack)
						} else {
							log.Printf("...package '%v' found in partially-downloaded repo, recyclable, will be skipped\n", pack.Location.Href)
						}
					}
				} else {
					log.Printf("...package '%v' found in already-downloaded repo, recyclable, will be recycled\n", pack.Location.Href)
					packagesToRecycle = append(packagesToRecycle, pack)
				}
			}
		}
		return
	})
	return
}
