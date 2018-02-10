package get

import (
	"crypto"
	"io"
	"log"
	"os"
	"path"

	"github.com/moio/minima/util"
)

// FileStorage allows to store data in a local directory
type FileStorage struct {
	directory      string
	checksumBuffer []byte
}

// NewFileStorage returns a new Storage given a local directory
func NewFileStorage(directory string) Storage {
	return &FileStorage{directory, make([]byte, 4*1024*1024)}
}

// NewReader returns a Reader for a file in the permanent location, returns ErrFileNotFound
// if the requested path was not found at all
func (s *FileStorage) NewReader(filename string) (reader io.ReadCloser, err error) {
	fullPath := path.Join(s.directory, filename)
	stat, err := os.Stat(fullPath)
	if os.IsNotExist(err) || stat == nil {
		err = ErrFileNotFound
		return
	}

	f, err := os.Open(fullPath)
	if err != nil {
		log.Fatal(err)
	}

	return f, err
}

// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
func (s *FileStorage) StoringMapper(filename string, checksum string, hash crypto.Hash) util.ReaderMapper {
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

		result = util.NewTeeReadCloser(reader, util.NewChecksummingWriter(file, checksum, hash))
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
