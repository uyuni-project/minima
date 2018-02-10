package get

import (
	"crypto"
	"errors"

	"github.com/moio/minima/util"
)

// Storage allows to store data in the form of files. Files are accumulated in
// a "temporary" location until Commit is called at that point any file in the
// temporary location is moved in the "permanent" location
type Storage interface {
	// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
	StoringMapper(filename string, checksum string) util.ReaderMapper
	// Commit moves any temporary file accumulated so far to the permanent location
	Commit() (err error)
	// Checksum returns the checksum value of a file in the permanent location, according to the checksumType algorithm
	// returns ErrFileNotFound if the requested path was not found at all
	Checksum(filename string, hash crypto.Hash) (checksum string, err error)
	// Recycle will copy a file from the permanent to the temporary location
	Recycle(filename string) (err error)
}

// ErrFileNotFound signals that the requested file was not found
var ErrFileNotFound = errors.New("File not found")
