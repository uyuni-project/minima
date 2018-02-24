package get

import (
	"crypto"
	"errors"
	"io"

	"github.com/moio/minima/util"
)

// Location represents a directory in a Storage object
type Location int

const (
	// Permanent represents a location for fully and correctly synced repos
	Permanent Location = iota
	// Temporary represents a temporary location while downloading
	Temporary
)

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
var ErrFileNotFound = errors.New("File not found")
