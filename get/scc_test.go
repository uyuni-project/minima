package get

import (
	"fmt"
	"net/http"
	"testing"
)

func TestSCCURLs(t *testing.T) {
	// Respond to http://localhost:8080/test with "Hello, World"
	http.HandleFunc("/connect/organizations/repositories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "<http://localhost:8080/connect/organizations/repositories2>; rel=\"next\"")
		fmt.Fprintf(w, "[{\"url\" : \"test\", \"name\" : \"test\", \"description\" : \"test\"}]")
	})

	http.HandleFunc("/connect/organizations/repositories2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", "")
		fmt.Fprintf(w, "[{\"url\" : \"test2\", \"name\" : \"test2\", \"description\" : \"test2\"}]")
	})

	urls, err := SCCURLs("http://localhost:8080", "user", "pass", []string{"test2"}, []string{""})
	if err != nil {
		t.Error(err)
	}

	if len(urls) != 1 {
		t.Error("expected 1 url")
	}

	if urls[0] != "test2" {
		t.Error("expected test2, got " + urls[0])
	}
}
