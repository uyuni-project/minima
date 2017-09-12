package util

import (
	"io"
)

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

// ReaderFunction maps a Reader to some other object
type ReaderFunction func(io.ReadCloser) (result interface{}, err error)

// NopReaderFunction maps a Reader to nothing
func NopReaderFunction(r io.ReadCloser) (result interface{}, err error) {
	return
}

// ReaderMapper maps a Reader to another Reader
type ReaderMapper func(io.ReadCloser) (result io.ReadCloser, err error)
