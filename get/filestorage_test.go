package get

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uyuni-project/minima/util"
)

func TestFileStorageRejectsEscapingPaths(t *testing.T) {
	base := t.TempDir()
	storage := NewFileStorage(filepath.Join(base, "repo"))

	for _, name := range []string{"../escaped", "/escaped", "x86_64/../../escaped"} {
		_, err := storage.StoringMapper(name, "", 0)(io.NopCloser(strings.NewReader("data")))
		if err == nil {
			t.Errorf("StoringMapper accepted %q", name)
		}
		if _, err := storage.NewReader(name, Permanent); err == nil {
			t.Errorf("NewReader accepted %q", name)
		}
		if err := storage.Recycle(name); err == nil {
			t.Errorf("Recycle accepted %q", name)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "escaped")); err == nil {
		t.Error("file was written outside the storage directory")
	}

	// a regular repo-relative path still works
	err := util.Compose(storage.StoringMapper("x86_64/ok.rpm", "", 0), util.Nop)(io.NopCloser(strings.NewReader("data")))
	if err != nil {
		t.Error(err)
	}
}
