package get

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
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

func TestDownloadApply(t *testing.T) {
	err := serveTestString()
	if err != nil {
		t.Error(err)
	}

	err = DownloadApply("http://localhost:8080/test", func(reader io.ReadCloser) (err error) {
		result, err := ioutil.ReadAll(reader)
		if err != nil {
			t.Error(err)
		}
		if string(result) != "Hello, World" {
			t.Error("Unexpected value ", result)
		}
		return
	})
	if err != nil {
		t.Error(err)
	}
}
