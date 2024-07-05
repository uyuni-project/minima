package get

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/uyuni-project/minima/util"
	"golang.org/x/crypto/openpgp"
)

// common

// XMLLocation maps a <location> tag in repodata/repomd.xml or repodata/<ID>-primary.xml.<compression>
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

// repodata/<ID>-primary.xml.<compression>

// XMLMetaData maps a <metadata> tag in repodata/<ID>-primary.xml.<compression>
type XMLMetaData struct {
	Packages []XMLPackage `xml:"package"`
}

// XMLPackage maps a <package> tag in repodata/<ID>-primary.xml.<compression>
type XMLPackage struct {
	Arch     string      `xml:"arch"`
	Location XMLLocation `xml:"location"`
	Checksum XMLChecksum `xml:"checksum"`
}

// XMLChecksum maps a <checksum> tag in repodata/<ID>-primary.xml.<compression>
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
const releasePath = "Release"

type RepoType struct {
	MetadataPath string
	PackagesType string
	DecodeMetadata func(io.Reader) (XMLRepomd, error)
	DecodePackages func(io.Reader, string) (XMLMetaData, error)
	MetadataSignatureExt string
    Noarch string
}

var (
repoTypes = map[string]RepoType {
	"rpm": RepoType {
		MetadataPath: "repodata/repomd.xml",
		PackagesType: "primary",
		DecodeMetadata: func (reader io.Reader) (repomd XMLRepomd, err error) {
			decoder := xml.NewDecoder(reader)
			err = decoder.Decode(&repomd)
			return
		},
		DecodePackages: readMetaData,
		MetadataSignatureExt: ".asc",
        Noarch: "noarch",
	},
	"deb": RepoType {
		MetadataPath: "Release",
		PackagesType: "Packages",
		DecodeMetadata: decodeRelease,
		DecodePackages: decodePackages,
		MetadataSignatureExt: ".gpg",
        Noarch: "all",
	},
}
Legacy bool
)
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
			sc := uerr.StatusCode
			if sc == 401 || sc == 403 || sc == 404 || sc == 410 || sc == 502 || sc == 503 || sc == 504 {
				log.Printf("Got %v, presumably temporarily, retrying...\n", sc)
			} else {
				return err
			}
		}

		_, checksumError := err.(*util.ChecksumError)
		if checksumError {
			log.Printf(err.Error())
			log.Printf("Checksum did not match, presumably the repo was published while syncing, retrying...\n")
			continue
		}

		_, signatureError := err.(*SignatureError)
		if signatureError {
			log.Printf(err.Error())
			log.Printf("Signature not valid, presumably the repo was published while syncing, retrying...\n")
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
	//log.Printf("SYNCER: %v\n", r)
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
	doProcessMetadata := func (reader io.ReadCloser, repoType RepoType) (err error) {
		b, err := ioutil.ReadAll(reader)
		if err != nil {
			return
		}

		err = r.checkRepomdSignature(bytes.NewReader(b), repoType)
		if err != nil {
			return
		}

		repomd, err := repoType.DecodeMetadata(bytes.NewReader(b))
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

			if data[i].Type == repoType.PackagesType {
				packagesToDownload, packagesToRecycle, err = r.processPrimary(metadataLocation, checksumMap, repoType)
			}
		}
		return
	}

	err = r.downloadStoreApply(repomdPath, "", path.Base(repomdPath), 0, func(reader io.ReadCloser) (err error) {
		err = doProcessMetadata(reader, repoTypes["rpm"])
		return
	})
	if err != nil {
		log.Printf(err.Error())
		log.Printf("Fallback to next repo type")
		// attempt to download Debian's Release file
		err = r.downloadStoreApply(releasePath, "", path.Base(releasePath), 0, func(reader io.ReadCloser) (err error) {
			err = doProcessMetadata(reader, repoTypes["deb"])
			return
		})
		return
	}

	return
}

func (r *Syncer) checkRepomdSignature(repomdReader io.Reader, repoType RepoType) (err error) {
	ascPath := repoType.MetadataPath + repoType.MetadataSignatureExt
	keyPath := repoType.MetadataPath + ".key"

	err = r.downloadStoreApply(ascPath, "", path.Base(ascPath), 0, func(signatureReader io.ReadCloser) (err error) {
		err = r.downloadStoreApply(keyPath, "", path.Base(keyPath), 0, func(keyReader io.ReadCloser) (err error) {
			keyring, err := openpgp.ReadArmoredKeyRing(keyReader)
			if err != nil {
				return &SignatureError{keyPath + " file does not contain a valid signature"}
			}
			_, err = openpgp.CheckArmoredDetachedSignature(keyring, repomdReader, signatureReader)
			if err != nil {
				return &SignatureError{ascPath + " signature check failed, signature is not valid"}
			}
			return
		})
		if err != nil {
			uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
			if unexpectedStatusCode && uerr.StatusCode == 404 {
				log.Printf("Got 404, ignoring...")
				err = nil
			}
		}
		return
	})
	if err != nil {
		uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
		if unexpectedStatusCode && uerr.StatusCode == 404 {
			log.Printf("Got 404, ignoring...")
			err = nil
		}
	}
	return
}

// SignatureError is returned if a signature was found but it's invalid
type SignatureError struct {
	reason string
}

func (e *SignatureError) Error() string {
	return fmt.Sprintf("Signature error: %s", e.reason)
}

// Uncompress and read primary XML
func readMetaData(reader io.Reader, compType string) (XMLMetaData, error) {
	var primary XMLMetaData
	switch compType {
	case "gz":
		reader, err := gzip.NewReader(reader)
		if err != nil {
			return primary, err
		}
		defer reader.Close()

		decoder := xml.NewDecoder(reader)
		err = decoder.Decode(&primary)
	case "zst":
		reader, err := zstd.NewReader(reader)
		if err != nil {
			return primary, err
		}
		defer reader.Close()

		decoder := xml.NewDecoder(reader)
		err = decoder.Decode(&primary)
	default:
		return primary, errors.New("Unsupported compression type")
	}
	return primary, nil
}


func (r *Syncer) readChecksumMap() (checksumMap map[string]XMLChecksum) {
	checksumMap = make(map[string]XMLChecksum)
	repomdReader, err := r.storage.NewReader(repomdPath, Permanent)
	repoType := repoTypes["rpm"]
	if err != nil {
		if err == ErrFileNotFound {
			repomdReader, err = r.storage.NewReader(releasePath, Permanent)
			if err != nil {
				log.Println("First-time sync started")
			} else {
				repoType = repoTypes["deb"]
			}
		} else {
			log.Println("Error while reading previously-downloaded metadata. Starting sync from scratch")
		}
		return
	}
	defer repomdReader.Close()

	repomd, err := repoType.DecodeMetadata(repomdReader)
	if err != nil {
		log.Println("Error while parsing previously-downloaded metadata. Starting sync from scratch")
		return
	}

	data := repomd.Data
	for i := 0; i < len(data); i++ {
		dataHref := data[i].Location.Href
		dataChecksum := data[i].Checksum
		checksumMap[dataHref] = dataChecksum
		if data[i].Type == repoType.PackagesType {
			primaryReader, err := r.storage.NewReader(dataHref, Permanent)
			if err != nil {
				return
			}
			compType := strings.Trim(filepath.Ext(dataHref), ".")
			primary, err := repoType.DecodePackages(primaryReader, compType)
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
func (r *Syncer) processPrimary(path string, checksumMap map[string]XMLChecksum, repoType RepoType) (packagesToDownload []XMLPackage, packagesToRecycle []XMLPackage, err error) {
	reader, err := r.storage.NewReader(path, Temporary)
	if err != nil {
		return
	}
	compType := strings.Trim(filepath.Ext(path), ".")
	primary, err := repoType.DecodePackages(reader, compType)
	if err != nil {
		return
	}

	allArchs := len(r.archs) == 0
	for _, pack := range primary.Packages {
		if allArchs || pack.Arch == repoType.Noarch || r.archs[pack.Arch] || (r.archs["i586"] && pack.Arch == "i686") || (Legacy && (r.archs["x86_64"] && (pack.Arch == "i586" || pack.Arch == "i686"))) {
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
	previousChecksum, foundInChecksumMap := checksumMap[location]

	if foundInChecksumMap {
		reader, err := r.storage.NewReader(location, Permanent)
		if err != nil {
			return Download
		}
		defer reader.Close()
	}

	if !foundInChecksumMap || previousChecksum.Type != checksum.Type || previousChecksum.Checksum != checksum.Checksum {
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

// Functions to handle Debian formatted repositories

func decodeRelease(reader io.Reader) (repomd XMLRepomd, err error) {
	entries, err := util.ProcessPropertiesFile(reader)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		err = errors.New("No content in Release file")
		return
	}
	if len(entries[0]["SHA256"]) == 0 {
		err = errors.New("Missing SHA256 entry in Release file")
		return
	}
	fileEntries := strings.Split(entries[0]["SHA256"], "\n")

	data := make([]XMLData, 0)
	for _, fileEntry := range fileEntries {
		infos := strings.Split(fileEntry, " ")
		if len(infos) != 3 {
			err = fmt.Errorf("badly formatted file entry: '%s'", fileEntry)
			return
		}
		fileData := XMLData{
			Type: infos[2],
			Location: XMLLocation{ Href: infos[2] },
			Checksum: XMLChecksum{ Type: "sha256", Checksum: infos[0] },
		}
		data = append(data, fileData)
	}
	repomd = XMLRepomd{Data: data}
	return
}

func decodePackages(reader io.Reader, _ string) (metadata XMLMetaData, err error) {
	packagesEntries, err := util.ProcessPropertiesFile(reader)
	if err != nil {
		return
	}

	packages := make([]XMLPackage, 0)
	for _, packageEntry := range packagesEntries {
		packages = append(packages, XMLPackage{
			Arch: packageEntry["Architecture"],
			Location: XMLLocation{ Href: packageEntry["Filename"] },
			Checksum: XMLChecksum{ Type: "sha256", Checksum: packageEntry["SHA256"] },
		})
	}
	metadata = XMLMetaData{ Packages: packages }
	return
}
