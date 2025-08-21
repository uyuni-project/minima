package storage

import (
	"crypto"
	"errors"
	"io"
	"net/url"
	"path/filepath"

	"github.com/uyuni-project/minima/util"
)

// Location represents a directory in a Storage object
type Location int

const (
	// Permanent represents a location for fully and correctly synced repos
	Permanent Location = iota
	// Temporary represents a temporary location while downloading
	Temporary
)

type StorageConfig struct {
	Type string
	// file-specific
	Path string
	// s3-specific
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	Region          string
	Bucket          string
	JsonPath        string `yaml:"jsonpath"`
	ProjectID       string `yaml:"projectid"`
}

// Storage allows to store data in the form of files. Files are accumulated in
// a "temporary" location until Commit is called at that point any file in the
// temporary location is moved in the "permanent" location
type Storage interface {
	// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
	StoringMapper(filename string, checksum string, hash crypto.Hash) util.ReaderMapper
	// Commit moves any temporary file accumulated so far to the permanent location
	Commit() (err error)
	// NewReader returns a Reader for a file in a location, returns ErrFileNotFound
	// if the requested path was not found at all
	NewReader(filename string, location Location) (reader io.ReadCloser, err error)
	// Recycle will copy a file from the permanent to the temporary location
	Recycle(filename string) (err error)
}

// ErrFileNotFound signals that the requested file was not found
var ErrFileNotFound = errors.New("file not found")

// ErrInvalidStorageType signals that the storage type is not supported
var ErrInvalidStorageType = errors.New("invalid storage type")

// StorageFromConfig returns the Storage implementation defined in the .yaml configuration
//
// Returns an error if the storage type is not supported or it was not possible to initialize it correctly
func FromConfig(config StorageConfig, repoURL *url.URL) (Storage, error) {
	switch config.Type {
	case "file":
		return NewFileStorage(filepath.Join(config.Path, filepath.FromSlash(repoURL.Path))), nil
	case "s3":
		return NewS3Storage(config.AccessKeyID, config.AccessKeyID, config.Region, config.Bucket+repoURL.Path)
	default:
		return nil, ErrInvalidStorageType
	}
}
