package util

import (
	"io"
	"io/ioutil"
)

// ReaderFunction maps a Reader to some other object
type ReaderFunction func(io.ReadCloser) (result interface{}, err error)

// ReaderMapper maps a Reader to another Reader
type ReaderMapper func(io.ReadCloser) (result io.ReadCloser, err error)

// Compose composes a ReaderFunction with a ReaderMapper
func Compose(mapper ReaderMapper, f ReaderFunction) ReaderFunction {
	return func(r io.ReadCloser) (result interface{}, err error) {
		mappedReader, err := mapper(r)
		if err != nil {
			return
		}
		defer mappedReader.Close()

		return f(mappedReader)
	}
}

// NopReaderFunction maps a Reader to nothing
func NopReaderFunction(r io.ReadCloser) (result interface{}, err error) {
	return
}

// StringReaderFunction maps a Reader to a string
func StringReaderFunction(r io.ReadCloser) (result interface{}, err error) {
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return
	}
	result = string(bytes)
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
