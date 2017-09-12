package get

import (
	"github.com/moio/minima/util"
)

type Storage interface {
	StoringMapper(filename string) util.ReaderMapper
	FileExists(filename string) (fileExists bool)
	Checksum(filename string, checksumType ChecksumType) (checksum string, err error)
}

type ChecksumType int

const (
	SHA1 ChecksumType = iota
	SHA256
)
