package get

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadURL(t *testing.T) {
	server := httptest.NewServer(http.DefaultServeMux)
	defer server.Close()

	// Respond to /test with "Hello, World"
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World")
	})

	// 200
	reader, err := ReadURL(server.URL + "/test")
	if err != nil {
		t.Error(err)
	}

	result, err := io.ReadAll(reader)
	if err != nil {
		t.Error(err)
	}
	if string(result) != "Hello, World" {
		t.Error("Unexpected value ", result)
	}

	// 404
	_, err = ReadURL(server.URL + "/not_existing")

	uerr, unexpected := err.(*UnexpectedStatusCodeError)
	if !unexpected {
		t.Error("404 error expected, got ", err)
	}

	if uerr.StatusCode != 404 {
		t.Error("404 error expected, got ", uerr.StatusCode)
	}
}
