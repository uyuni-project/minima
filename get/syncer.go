package get

import (
	"bytes"
	"compress/gzip"
	"crypto"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/klauspost/compress/zstd"
	"github.com/uyuni-project/minima/util"
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
	"sha512": crypto.SHA512,
}

type RepoType struct {
	Name                   string
	MetadataPath           string
	PackagesType           string
	DecodeMetadata         func(io.Reader, []string) (XMLRepomd, error)
	DecodePackages         func(io.Reader, string) (XMLMetaData, error)
	MetadataSignatureExt   string
	Noarch                 string
	RequiredKeys           []string
	MandatoryMetadataPaths []string
	OptionalMetadataPaths  []string
}

var (
	packageExtensions = map[string]struct{}{
		".rpm":  {},
		".deb":  {},
		".udeb": {},
	}
	repoTypes = []RepoType{
		{
			Name:         "rpm",
			MetadataPath: "repodata/repomd.xml",
			PackagesType: "primary",
			DecodeMetadata: func(reader io.Reader, _ []string) (repomd XMLRepomd, err error) {
				decoder := xml.NewDecoder(reader)
				err = decoder.Decode(&repomd)
				return
			},
			DecodePackages:       readMetaData,
			MetadataSignatureExt: ".asc",
			Noarch:               "noarch",
		},
		{
			Name:                  "deb",
			MetadataPath:          "Release",
			PackagesType:          "Packages",
			DecodeMetadata:        decodeRelease,
			DecodePackages:        decodePackages,
			MetadataSignatureExt:  ".gpg",
			Noarch:                "all",
			RequiredKeys:          []string{"SHA256"},
			OptionalMetadataPaths: []string{"InRelease"},
		},
	}
)

// Syncer syncs repos from an HTTP source to a Storage
type Syncer struct {
	// URL of the repo this syncer syncs
	URL         url.URL
	archs       map[string]bool
	storage     Storage
	quiet       bool
	skipLegacy  bool
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

// NewSyncer creates a new Syncer
func NewSyncer(url url.URL, archs map[string]bool, storage Storage, quiet bool, skipLegacy bool) *Syncer {
	return &Syncer{url, archs, storage, quiet, skipLegacy}
}

// StoreRepo stores an HTTP repo in a Storage, automatically retrying in case of recoverable errors
func (r *Syncer) StoreRepo() (err error) {
	checksumMap := r.readChecksumMap()
	for range 20 {
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
			log.Println(err.Error())
			log.Println("Checksum did not match, presumably the repo was published while syncing, retrying...")
			continue
		}

		_, signatureError := err.(*SignatureError)
		if signatureError {
			log.Println(err.Error())
			log.Println("Signature not valid, presumably the repo was published while syncing, retrying...")
		} else {
			return err
		}
	}

	log.Println("Too many temporary errors, aborting...")
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
		// we need to escape package names because some CDN, proxies (...) are not perfectly RFC 3986 compliant
		// in such cases characters like '+' (which are common in c++ pkgs) will assume a different meaning
		name := path.Base(pack.Location.Href)
		escapedName := url.QueryEscape(name)
		relativeURL := strings.TrimSuffix(pack.Location.Href, name) + escapedName

		description := fmt.Sprintf("(%v/%v) %v", i+1, downloadCount, name)
		err = r.downloadStoreApply(relativeURL, pack.Checksum.Checksum, description, hashMap[pack.Checksum.Type], util.Nop)
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

	log.Println("Committing changes...")
	err = r.storage.Commit()
	if err != nil {
		return
	}
	return
}

// downloadStoreApply downloads a repo-relative path into a file, while applying a ReaderConsumer
func (r *Syncer) downloadStoreApply(relativePath string, checksum string, description string, hash crypto.Hash, f util.ReaderConsumer) error {
	if !r.quiet {
		log.Printf("Downloading %v...", description)
	}

	repoURL := r.URL
	repoURL.Path = path.Join(repoURL.Path, relativePath)
	finalURL := fmt.Sprintf("%s://%s%s?%s", repoURL.Scheme, repoURL.Host, repoURL.Path, repoURL.Query().Encode())

	body, err := ReadURL(finalURL)
	if err != nil {
		return err
	}
	// unescape to preserve original pkg name
	storagePath, err := url.QueryUnescape(relativePath)
	if err != nil {
		return err
	}
	return util.Compose(r.storage.StoringMapper(storagePath, checksum, hash), f)(body)
}

// processMetadata stores the repo metadata and returns a list of package file
// paths to download
func (r *Syncer) processMetadata(checksumMap map[string]XMLChecksum) (packagesToDownload []XMLPackage, packagesToRecycle []XMLPackage, err error) {
	doProcessMetadata := func(reader io.ReadCloser, repoType RepoType) (err error) {
		b, err := io.ReadAll(reader)
		if err != nil {
			return
		}

		err = r.checkRepomdSignature(bytes.NewReader(b), repoType)
		if err != nil {
			return
		}

		repomd, err := repoType.DecodeMetadata(bytes.NewReader(b), repoType.RequiredKeys)
		if err != nil {
			return
		}

		data := repomd.Data
		for _, entry := range data {
			if !r.quiet {
				log.Println(entry.Location.Href)
			}

			metadataLocation := entry.Location.Href
			metadataChecksum := entry.Checksum

			decision := r.decide(metadataLocation, metadataChecksum, checksumMap)
			switch decision {
			case Download:
				if !r.quiet {
					log.Println("...downloading")
				}

				err = r.downloadStoreApply(metadataLocation, metadataChecksum.Checksum, path.Base(metadataLocation), hashMap[metadataChecksum.Type], util.Nop)
				if err != nil {
					return
				}
			case Recycle:
				if !r.quiet {
					log.Println("...recycling")
				}

				r.storage.Recycle(metadataLocation)
			}

			if entry.Type == repoType.PackagesType {
				packagesToDownload, packagesToRecycle, err = r.processPrimary(metadataLocation, checksumMap, repoType)
			}
		}
		return
	}

	for _, repoType := range repoTypes {
		log.Printf("Trying repo type: %s\n", repoType.Name)
		err = r.downloadStoreApply(repoType.MetadataPath, "", path.Base(repoType.MetadataPath), 0, func(reader io.ReadCloser) error {
			return doProcessMetadata(reader, repoType)
		})
		if err != nil {
			log.Printf("Repo type %s failed: %v\n", repoType.Name, err)
			log.Println("Fallback to next repo type")
			continue
		}

		for _, mandatoryPath := range repoType.MandatoryMetadataPaths {
			err = r.downloadStoreApply(mandatoryPath, "", mandatoryPath, 0, util.Nop)
			if err != nil {
				log.Printf("Failed to download mandatory file %s, retrying sync: %v \n", mandatoryPath, err)
				return
			}
		}

		for _, optionalPath := range repoType.OptionalMetadataPaths {
			optErr := r.downloadStoreApply(optionalPath, "", path.Base(optionalPath), 0, util.Nop)
			optErr = ignoreStatusCode(optErr, 403, 404)
			if optErr != nil {
				log.Printf("Failed to download optional file %s, retrying sync: %v\n", optionalPath, optErr)
				err = optErr
				return
			}
		}
		break
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
			_, err = openpgp.CheckArmoredDetachedSignature(keyring, repomdReader, signatureReader, nil)
			if err != nil {
				return &SignatureError{ascPath + " signature check failed, signature is not valid"}
			}
			return
		})
		if err != nil {
			err = ignoreStatusCode(err, 404)
		}
		return
	})
	if err != nil {
		err = ignoreStatusCode(err, 403, 404)
	}
	return
}

func ignoreStatusCode(err error, codes ...int) error {
	uerr, unexpectedStatusCode := err.(*UnexpectedStatusCodeError)
	if unexpectedStatusCode {
		for _, code := range codes {
			if uerr.StatusCode == code {
				log.Printf("Got %d, ignoring...\n", code)
				return nil
			}
		}
	}
	return err
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
		if err = decoder.Decode(&primary); err != nil {
			return primary, err
		}
	case "zst":
		reader, err := zstd.NewReader(reader)
		if err != nil {
			return primary, err
		}
		defer reader.Close()

		decoder := xml.NewDecoder(reader)
		if err = decoder.Decode(&primary); err != nil {
			return primary, err
		}
	default:
		return primary, errors.New("unsupported compression type")
	}

	return primary, nil
}

func (r *Syncer) readChecksumMap() (checksumMap map[string]XMLChecksum) {
	checksumMap = make(map[string]XMLChecksum)

	var repoType RepoType
	var repomdReader io.ReadCloser
	for _, rt := range repoTypes {
		var err error
		repomdReader, err = r.storage.NewReader(rt.MetadataPath, Permanent)
		if err == nil {
			repoType = rt
			break
		}
		if err != ErrFileNotFound {
			log.Println("Error while reading previously-downloaded metadata. Starting sync from scratch")
			return
		}
	}
	if repomdReader == nil {
		log.Println("First-time sync started")
		return
	}
	defer repomdReader.Close()

	repomd, err := repoType.DecodeMetadata(repomdReader, repoType.RequiredKeys)
	if err != nil {
		log.Println("Error while parsing previously-downloaded metadata. Starting sync from scratch")
		return
	}

	data := repomd.Data
	for _, d := range data {
		dataHref := d.Location.Href
		dataChecksum := d.Checksum
		checksumMap[dataHref] = dataChecksum
		if d.Type == repoType.PackagesType {
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
		legacyPackage := (pack.Arch == "i586" || pack.Arch == "i686")

		if r.skipLegacy && legacyPackage {
			if !r.quiet {
				fmt.Println("Skipping legacy package:", pack.Location.Href)
			}
			continue
		}

		if allArchs || pack.Arch == repoType.Noarch || r.archs[pack.Arch] || (r.archs["x86_64"] && legacyPackage) {
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

		log.Printf("Reading %s checksum\n", checksum.Type)
		readChecksum, err := util.Checksum(reader, hashMap[checksum.Type])
		if err != nil || readChecksum != checksum.Checksum {
			return Download
		}
		return Skip
	}
	return Recycle
}

// Functions to handle Debian formatted repositories
func decodeRelease(reader io.Reader, requiredKeys []string) (repomd XMLRepomd, err error) {
	entries, err := util.ProcessPropertiesFile(reader)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		err = errors.New("no content in Release file")
		return
	}
	var fileListKey string
	for _, key := range requiredKeys {
		if len(entries[0][key]) > 0 {
			fileListKey = key
			break
		}
	}
	if fileListKey == "" {
		err = fmt.Errorf("missing required key(s) %v in Release file", requiredKeys)
		return
	}
	fileEntries := strings.Split(entries[0][fileListKey], "\n")

	data := make([]XMLData, 0)
	for _, fileEntry := range fileEntries {
		infos := strings.Split(fileEntry, " ")
		if len(infos) != 3 {
			err = fmt.Errorf("badly formatted file entry: '%s'", fileEntry)
			return
		}
		fileData := XMLData{
			Type:     infos[2],
			Location: XMLLocation{Href: infos[2]},
			Checksum: XMLChecksum{Type: "sha256", Checksum: infos[0]},
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
			Arch:     packageEntry["Architecture"],
			Location: XMLLocation{Href: packageEntry["Filename"]},
			Checksum: XMLChecksum{Type: "sha256", Checksum: packageEntry["SHA256"]},
		})
	}
	metadata = XMLMetaData{Packages: packages}
	return
}
