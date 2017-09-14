package get

import (
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
	// FileExists checks whether a file exists or not in the permanent location
	FileExists(filename string) (fileExists bool)
	// Checksum returns the checksum value of a file in the permanent location, according to the checksumType algorithm
	Checksum(filename string, checksumType ChecksumType) (checksum string, err error)
	// Recycle will copy a file from the permanent to the temporary location
	Recycle(filename string) (err error)
}

// ChecksumType is an enumeration of supported checksum algorithms
type ChecksumType int

const (
	// SHA1 identifier
	SHA1 ChecksumType = iota
	// SHA256 identifier
	SHA256
)
