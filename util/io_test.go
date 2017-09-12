package util

import (
	"bytes"
	"io"
	"testing"
)

func TestCompose(t *testing.T) {
	helloWorldReader := NewNopReadCloser(bytes.NewBufferString("Hello, World"))
	bangMapper := func(r io.ReadCloser) (result io.ReadCloser, err error) {
		s, err := StringReaderFunction(r)
		if err != nil {
			t.Error(err)
		}
		result = NewNopReadCloser(bytes.NewBufferString(s.(string) + "!"))
		return
	}

	composedReaderFunction := Compose(bangMapper, StringReaderFunction)
	result, err := composedReaderFunction(helloWorldReader)
	if err != nil {
		t.Error(err)
	}
	if result != "Hello, World!" {
		t.Error("Unexpected value ", result)
	}
}
