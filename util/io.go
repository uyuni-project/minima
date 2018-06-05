package util

import (
	"crypto"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
)

// ReaderConsumer consumes bytes from a Reader
type ReaderConsumer func(reader io.ReadCloser) (err error)

// ReaderMapper maps a Reader to another Reader
type ReaderMapper func(reader io.ReadCloser) (result io.ReadCloser, err error)

// Compose composes a ReaderConsumer with a ReaderMapper
func Compose(mapper ReaderMapper, f ReaderConsumer) ReaderConsumer {
	return func(reader io.ReadCloser) (err error) {
		mappedReader, err := mapper(reader)
		if err != nil {
			return
		}
		defer func() {
			if err != nil {
				mappedReader.Close()
			} else {
				err = mappedReader.Close()
			}
		}()

		return f(mappedReader)
	}
}

// Nop is a ReaderConsumer that does nothing
func Nop(reader io.ReadCloser) (err error) {
	return
}

// NopReadCloser wraps a Reader into a ReadCloser
type NopReadCloser struct{ r io.Reader }

// NewNopReadCloser returns a new NopReadCloser
func NewNopReadCloser(r io.Reader) *NopReadCloser {
	return &NopReadCloser{r}
}

// Read delegates to the wrapped Read function
func (r *NopReadCloser) Read(p []byte) (n int, err error) { return r.r.Read(p) }

// Close does nothing
func (r *NopReadCloser) Close() error { return nil }

// TeeReadCloser uses a TeeReader to copy data from a reader to a writer
type TeeReadCloser struct {
	reader    io.ReadCloser
	writer    io.WriteCloser
	teeReader io.Reader
}

// NewTeeReadCloser returns a new TeeReadCloser
func NewTeeReadCloser(reader io.ReadCloser, writer io.WriteCloser) *TeeReadCloser {
	teeReader := io.TeeReader(reader, writer)
	return &TeeReadCloser{reader, writer, teeReader}
}

// Read delegates to the TeeReader implementation
func (t *TeeReadCloser) Read(p []byte) (n int, err error) {
	return t.teeReader.Read(p)
}

var discardBuffer = make([]byte, 4*1024*1024)

// Close closes the internal reader and writer
func (t *TeeReadCloser) Close() (err error) {
	// read any remaining bytes from the teeReader (discarding them)
	io.CopyBuffer(ioutil.Discard, t.teeReader, discardBuffer)
	err = t.reader.Close()
	if err != nil {
		return
	}
	err = t.writer.Close()
	return
}

// ChecksummingWriter is a WriteCloser that checks on close that the checksum matches
type ChecksummingWriter struct {
	writer       io.WriteCloser
	expectedSum  string
	hashFunction crypto.Hash
	hash         hash.Hash
}

// NewChecksummingWriter returns a new ChecksummingWriter
func NewChecksummingWriter(writer io.WriteCloser, expectedSum string, hashFunction crypto.Hash) *ChecksummingWriter {
	if hashFunction != 0 {
		return &ChecksummingWriter{writer, expectedSum, hashFunction, hashFunction.New()}
	}
	return &ChecksummingWriter{writer, expectedSum, hashFunction, nil}
}

// Write delegates to the writer and hash
func (w *ChecksummingWriter) Write(p []byte) (n int, err error) {
	if w.hashFunction != 0 {
		w.hash.Write(p)
	}
	return w.writer.Write(p)
}

// Close delegates to the writer and checks the hash sum
func (w *ChecksummingWriter) Close() (err error) {
	err = w.writer.Close()
	if err != nil {
		return
	}
	if w.hashFunction != 0 {
		actualSum := hex.EncodeToString(w.hash.Sum(nil))
		if w.expectedSum != actualSum {
			err = &ChecksumError{w.expectedSum, actualSum}
		}
	}
	return
}

// ChecksumError is returned if the expected and actual checksums do not match
type ChecksumError struct {
	expected string
	actual   string
}

func (e *ChecksumError) Error() string {
	return fmt.Sprintf("Checksum mismatch: expected %s, actual %s", e.expected, e.actual)
}

// Checksum returns the checksum value from a Reader
func Checksum(reader io.ReadCloser, hash crypto.Hash) (checksum string, err error) {
	checksumBuffer := make([]byte, 4*1024*1024)

	h := hash.New()
	if _, err = io.CopyBuffer(h, reader, checksumBuffer); err != nil {
		return
	}
	checksum = hex.EncodeToString(h.Sum(nil))
	return
}
