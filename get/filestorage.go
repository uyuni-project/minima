package get

import (
	"bufio"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/moio/minima/util"
)

// Storage allows to store data in a local directory
type FileStorage struct {
	directory string
}

// NewStorage returns a new Storage given a local directory
func NewFileStorage(directory string) Storage {
	return &FileStorage{directory}
}

// FileExists returns a flag whether the file exists or not on the storage
func (s *FileStorage) FileExists(filename string) (fileExists bool) {
	fullPath := path.Join(s.directory, filename)
	file, err := os.Stat(fullPath)
	if os.IsNotExist(err) || file == nil {
		log.Printf("...package '%v' does not exists\n", fullPath)
		return false
	}
	return true
}

// Checksum returns the checksum value of a file,
// the algorithm to evaluate it depends on the checksumType parameter
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
		if _, err := io.Copy(h, f); err != nil {
			log.Fatal(err)
		}
		checksum = hex.EncodeToString(h.Sum(nil))
	case SHA256:
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			log.Fatal(err)
		}
		checksum = hex.EncodeToString(h.Sum(nil))
	default:
		err = errors.New("Unknown ChecksumType")
	}
	return
}

// StoringMapper returns a mapper that will store any read data to filename
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

		writer := bufio.NewWriter(file)
		teeReader := io.TeeReader(reader, writer)

		result = &storingReader{reader, writer, teeReader}
		return
	}
}

// storingReader uses a TeeReader to copy data from a reader to a writer
type storingReader struct {
	reader    io.ReadCloser
	writer    *bufio.Writer
	teeReader io.Reader
}

// Read delegates to the TeeReader implementation
func (t *storingReader) Read(p []byte) (n int, err error) {
	return t.teeReader.Read(p)
}

// Closes the internal reader and flushes the writer
func (t *storingReader) Close() (err error) {
	ioutil.ReadAll(t.teeReader)
	err = t.reader.Close()
	if err != nil {
		return
	}
	return t.writer.Flush()
}

func (s *FileStorage) Recycle(filename string) (err error) {
	newPath := path.Join(s.directory+"-in-progress", filename)
	err = os.MkdirAll(path.Dir(newPath), os.ModePerm)
	if err != nil {
		return
	}

	return os.Link(path.Join(s.directory, filename), newPath)
}

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
