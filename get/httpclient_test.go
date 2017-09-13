package get

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
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
		http.Serve(listener, nil)
	}()

	return <-errs
}

func TestDownloadApply(t *testing.T) {
	err := serveTestString()
	if err != nil {
		t.Error(err)
	}

	// 200
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

	// 404
	err = DownloadApply("http://localhost:8080/not_existing", util.Nop)
	if !strings.Contains(err.Error(), "404") {
		t.Error("404 error expected, got ", err)
	}
}
