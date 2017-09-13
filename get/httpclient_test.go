package get

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/moio/minima/util"
)

func TestDownloadApply(t *testing.T) {
	// Respond to http://localhost:8080/test with "Hello, World"
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, World")
	})

	// 200
	err := DownloadApply("http://localhost:8080/test", func(reader io.ReadCloser) (err error) {
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
