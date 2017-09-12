package util

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

func TestCompose(t *testing.T) {
	helloWorldReader := NewNopReadCloser(bytes.NewBufferString("Hello, World"))
	bangMapper := func(r io.ReadCloser) (result io.ReadCloser, err error) {
		raw, err := ioutil.ReadAll(r)
		if err != nil {
			t.Error(err)
		}
		result = NewNopReadCloser(bytes.NewBufferString(string(raw) + "!"))
		return
	}

	composedReaderFunction := Compose(bangMapper, func(reader io.ReadCloser) (err error) {
		result, err := ioutil.ReadAll(reader)
		if err != nil {
			t.Error(err)
		}
		if string(result) != "Hello, World!" {
			t.Error("Unexpected value ", result)
		}
		return
	})

	err := composedReaderFunction(helloWorldReader)
	if err != nil {
		t.Error(err)
	}
}
