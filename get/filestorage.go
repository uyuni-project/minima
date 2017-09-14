package get

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"path"

	"github.com/moio/minima/util"
)

// FileStorage allows to store data in a local directory
type FileStorage struct {
	directory string
}

// NewFileStorage returns a new Storage given a local directory
func NewFileStorage(directory string) Storage {
	return &FileStorage{directory}
}

// FileExists checks whether a file exists or not in the permanent location
func (s *FileStorage) FileExists(filename string) (fileExists bool) {
	fullPath := path.Join(s.directory, filename)
	file, err := os.Stat(fullPath)
	if os.IsNotExist(err) || file == nil {
		log.Printf("...package '%v' does not exists\n", fullPath)
		return false
	}
	return true
}

// Checksum returns the checksum value of a file in the permanent location, according to the checksumType algorithm
func (s *FileStorage) Checksum(filename string, checksumType ChecksumType) (checksum string, err error) {
	fullPath := path.Join(s.directory, filename)
	f, err := os.Open(fullPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	switch checksumType {
	case SHA1:
		h := sha1.New()
		if _, err = io.Copy(h, f); err != nil {
			log.Fatal(err)
		}
		checksum = hex.EncodeToString(h.Sum(nil))
	case SHA256:
		h := sha256.New()
		if _, err = io.Copy(h, f); err != nil {
			log.Fatal(err)
		}
		checksum = hex.EncodeToString(h.Sum(nil))
	default:
		err = errors.New("Unknown ChecksumType")
	}
	return
}

// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
func (s *FileStorage) StoringMapper(filename string) util.ReaderMapper {
	return func(reader io.ReadCloser) (result io.ReadCloser, err error) {
		fullPath := path.Join(s.directory+"-in-progress", filename)
		// attempt to create any missing directories in the full path
		err = os.MkdirAll(path.Dir(fullPath), os.ModePerm)
		if err != nil {
			return
		}

		file, err := os.Create(fullPath)
		if err != nil {
			return
		}

		result = util.NewTeeReadCloser(reader, file)
		return
	}
}

// Recycle will copy a file from the permanent to the temporary location
func (s *FileStorage) Recycle(filename string) (err error) {
	newPath := path.Join(s.directory+"-in-progress", filename)
	err = os.MkdirAll(path.Dir(newPath), os.ModePerm)
	if err != nil {
		return
	}

	return os.Link(path.Join(s.directory, filename), newPath)
}

// Commit moves any temporary file accumulated so far to the permanent location
func (s *FileStorage) Commit() (err error) {
	err = os.RemoveAll(s.directory + "-old")
	if err != nil {
		return
	}
	err = os.Rename(s.directory, s.directory+"-old")
	if err != nil && !os.IsNotExist(err) {
		return
	}
	err = os.Rename(s.directory+"-in-progress", s.directory)
	if err != nil {
		return
	}
	return os.RemoveAll(s.directory + "-old")
}
