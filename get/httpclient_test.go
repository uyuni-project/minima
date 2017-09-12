package get

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"testing"

	"github.com/moio/minima/util"
)

// Runs a server on http://localhost:8080/test responding with "Hello, World"
func serveTestString() error {
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World")
	})

	errs := make(chan error)
	go func() {
		listener, err := net.Listen("tcp", ":8080")
		errs <- err
		go http.Serve(listener, nil)
	}()

	return <-errs
}

func TestMain(m *testing.M) {
	err := serveTestString()
	if err != nil {
		log.Fatal(err)
	}
	m.Run()
}

func TestApply(t *testing.T) {
	result, err := Apply(func(r io.ReadCloser) (result interface{}, err error) {
		bytes, err := ioutil.ReadAll(r)
		result = string(bytes) + "!"
		return
	}, "http://localhost:8080/test")

	if err != nil {
		t.Error(err)
	}

	if result != "Hello, World!" {
		t.Error("Unexpected value ", result)
	}
}

func TestApplyStoring(t *testing.T) {
	storage := new(bytes.Buffer)

	result, err := ApplyStoring(func(r io.ReadCloser) (result interface{}, err error) {
		bytes, err := ioutil.ReadAll(r)
		result = string(bytes) + "!"
		return
	}, "http://localhost:8080/test", func(r io.ReadCloser) (result io.ReadCloser, err error) {
		return util.NewNopReadCloser(io.TeeReader(r, storage)), nil
	})

	if err != nil {
		t.Error(err)
	}

	if result != "Hello, World!" {
		t.Error("Unexpected value ", result)
	}

	if storage.String() != "Hello, World!" {
		t.Error("Unexpected value ", result)
	}
}
