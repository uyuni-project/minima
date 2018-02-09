package get

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestReadURL(t *testing.T) {
	// Respond to http://localhost:8080/test with "Hello, World"
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World")
	})

	// 200
	reader, err := ReadURL("http://localhost:8080/test")
	if err != nil {
		t.Error(err)
	}

	result, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Error(err)
	}
	if string(result) != "Hello, World" {
		t.Error("Unexpected value ", result)
	}

	// 404
	_, err = ReadURL("http://localhost:8080/not_existing")

	uerr, unexpected := err.(*UnexpectedStatusCodeError)
	if !unexpected {
		t.Error("404 error expected, got ", err)
	}

	if uerr.StatusCode != 404 {
		t.Error("404 error expected, got ", uerr.StatusCode)
	}
}
