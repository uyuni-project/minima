package get

import (
	"fmt"
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

func TestGetApply(t *testing.T) {
	err := serveTestString()
	if err != nil {
		t.Error(err)
	}

	result, err := GetApply("http://localhost:8080/test", util.StringReaderFunction)
	if err != nil {
		t.Error(err)
	}

	if result != "Hello, World" {
		t.Error("Unexpected value ", result)
	}
}
