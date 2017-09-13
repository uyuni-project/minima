package get

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRepo(t *testing.T) {
	// Respond to http://localhost:8080/repo serving the content of the testdata/repo directory
	http.Handle("/", http.FileServer(http.Dir("testdata")))

	directory := filepath.Join(os.TempDir(), "reposyncer_test")
	err := os.RemoveAll(directory)
	if err != nil {
		t.Error(err)
	}

	archs := map[string]bool{
		"x86_64": true,
	}
	storage := NewFileStorage(directory)
	reposyncer := NewRepoSyncer("http://localhost:8080/repo", archs, storage)

	// first sync
	err = reposyncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}

	expectedFiles := []string{
		filepath.Join("repodata", "0967c2f17755f88a7e8b2185a9b2a87d72cc3f10cc3574e3fcedd997e84db42b-updateinfo.xml.gz"),
		filepath.Join("repodata", "06288a8a3ec708ceabe3197087e9aa93cbf4d5b81a391857c0cbec9a676fdba2-filelists.xml.gz"),
		filepath.Join("repodata", "f08a89e1946493d15313244a30a2962e7a43fc07205b9f7fca1ebeec3c6d2d2e-other.xml.gz"),
		filepath.Join("repodata", "dadb7d32493327d1afdead2b4f191f8bcd449bcfe48fda241a0b94555c5495f6-primary.xml.gz"),
		filepath.Join("repodata", "repomd.xml"),
		filepath.Join("x86_64", "milkyway-dummy-2.0-1.1.x86_64.rpm"),
		filepath.Join("x86_64", "orion-dummy-1.1-1.1.x86_64.rpm"),
		filepath.Join("x86_64", "hoag-dummy-1.1-2.1.x86_64.rpm"),
		filepath.Join("x86_64", "perseus-dummy-1.1-1.1.x86_64.rpm"),
		filepath.Join("x86_64", "orion-dummy-sle12-1.1-4.1.x86_64.rpm"),
	}

	for _, file := range expectedFiles {
		originalInfo, err := os.Stat(filepath.Join("testdata", "repo", file))
		if err != nil {
			t.Error(err)
		}
		syncedInfo, err := os.Stat(filepath.Join(directory, file))
		if err != nil {
			t.Error(err)
		}
		if originalInfo.Size() != syncedInfo.Size() {
			t.Error("original and synced versions of", file, "differ:", originalInfo.Size(), "vs", syncedInfo.Size())
		}
	}

	// second sync
	err = reposyncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}

}
