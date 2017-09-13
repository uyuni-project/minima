package get

import (
	"log"
	"net"
	"net/http"
	"os"
	"testing"
)

// TestMain starts an HTTP server on localhost:8080 for test use
func TestMain(m *testing.M) {
	errs := make(chan error)
	go func() {
		listener, err := net.Listen("tcp", ":8080")
		errs <- err
		http.Serve(listener, nil)
	}()

	err := <-errs
	if err != nil {
		log.Panic("Could not start test HTTP server:", err)
	}

	os.Exit(m.Run())
}
