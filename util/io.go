package util

import (
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
		defer mappedReader.Close()

		return f(mappedReader)
	}
}

// Nop is a ReaderConsumer that does nothing
func Nop(reader io.ReadCloser) (err error) {
	defer reader.Close()
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

// Close closes the internal reader and writer
func (t *TeeReadCloser) Close() (err error) {
	ioutil.ReadAll(t.teeReader)
	err = t.reader.Close()
	if err != nil {
		return
	}
	err = t.writer.Close()
	return
}
