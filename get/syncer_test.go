package get

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/uyuni-project/minima/storage"
)

func TestStoreRepo(t *testing.T) {
	// Respond to http://localhost:8080/repo serving the content of the testdata/repo directory
	http.Handle("/", http.FileServer(http.Dir("testdata")))

	directory := filepath.Join(os.TempDir(), "syncer_test")
	err := os.RemoveAll(directory)
	if err != nil {
		t.Error(err)
	}

	archs := map[string]bool{
		"x86_64": true,
	}
	storage := storage.NewFileStorage(directory)
	url, err := url.Parse("http://localhost:8080/repo")
	if err != nil {
		t.Error(err)
	}
	syncer := NewSyncer(*url, archs, storage)

	// first sync
	err = syncer.StoreRepo()
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
		originalInfo, serr := os.Stat(filepath.Join("testdata", "repo", file))
		if err != nil {
			t.Fatal(serr)
		}
		syncedInfo, serr := os.Stat(filepath.Join(directory, file))
		if serr != nil {
			t.Fatal(serr)
		}
		if originalInfo.Size() != syncedInfo.Size() {
			t.Error("original and synced versions of", file, "differ:", originalInfo.Size(), "vs", syncedInfo.Size())
		}
	}

	// second sync
	err = syncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}
}

func TestStoreRepoZstd(t *testing.T) {
	directory := filepath.Join(os.TempDir(), "syncer_test")
	err := os.RemoveAll(directory)
	if err != nil {
		t.Error(err)
	}

	archs := map[string]bool{
		"x86_64": true,
	}
	storage := storage.NewFileStorage(directory)
	url, err := url.Parse("http://localhost:8080/zstrepo")
	if err != nil {
		t.Error(err)
	}
	syncer := NewSyncer(*url, archs, storage)

	// first sync
	err = syncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}

	expectedFiles := []string{
		filepath.Join("repodata", "106c411e443c6b97e7d547f78e32d6d2cdf8e80577999954fdb770d8a21581a0-filelists.xml.zst"),
		filepath.Join("repodata", "d7fd4cf502d9e1ab8865bc9690c34c3cadcfda0db269f4ddc9c6f026db3f2400-primary.xml.zst"),
		filepath.Join("repodata", "fa0767ea9359279bb6e8ec93a70cefb3863e4fa4bcbeebbfe755a5ce16c21b94-other.xml.zst"),
		filepath.Join("repodata", "repomd.xml"),
		filepath.Join("x86_64", "milkyway-dummy-2.0-1.1.x86_64.rpm"),
		filepath.Join("x86_64", "orion-dummy-1.1-1.1.x86_64.rpm"),
		filepath.Join("x86_64", "hoag-dummy-1.1-2.1.x86_64.rpm"),
		filepath.Join("x86_64", "perseus-dummy-1.1-1.1.x86_64.rpm"),
		filepath.Join("x86_64", "orion-dummy-sle12-1.1-4.1.x86_64.rpm"),
	}

	for _, file := range expectedFiles {
		originalInfo, serr := os.Stat(filepath.Join("testdata", "zstrepo", file))
		if err != nil {
			t.Fatal(serr)
		}
		syncedInfo, serr := os.Stat(filepath.Join(directory, file))
		if serr != nil {
			t.Fatal(serr)
		}
		if originalInfo.Size() != syncedInfo.Size() {
			t.Error("original and synced versions of", file, "differ:", originalInfo.Size(), "vs", syncedInfo.Size())
		}
	}

	// second sync
	err = syncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}
}

func TestStoreDebRepo(t *testing.T) {
	directory := filepath.Join(os.TempDir(), "syncer_test")
	err := os.RemoveAll(directory)
	if err != nil {
		t.Error(err)
	}

	archs := map[string]bool{
		"amd64": true,
	}

	storage := storage.NewFileStorage(directory)
	url, err := url.Parse("http://localhost:8080/deb_repo")
	if err != nil {
		t.Error(err)
	}
	syncer := NewSyncer(*url, archs, storage)

	// first sync
	err = syncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}

	expectedFiles := []string{
		"Packages.gz",
		"Release",
		filepath.Join("amd64", "milkyway-dummy_2.0-1.1_amd64.deb"),
		filepath.Join("amd64", "orion-dummy_1.1-1.1_amd64.deb"),
		filepath.Join("amd64", "hoag-dummy_1.1-2.1_amd64.deb"),
		filepath.Join("amd64", "perseus-dummy_1.1-1.1_amd64.deb"),
		filepath.Join("amd64", "orion-dummy-sle12_1.1-4.1_amd64.deb"),
	}

	for _, file := range expectedFiles {
		originalInfo, serr := os.Stat(filepath.Join("testdata", "deb_repo", file))
		if err != nil {
			t.Fatal(serr)
		}
		syncedInfo, serr := os.Stat(filepath.Join(directory, file))
		if serr != nil {
			t.Fatal(serr)
		}
		if originalInfo.Size() != syncedInfo.Size() {
			t.Error("original and synced versions of", file, "differ:", originalInfo.Size(), "vs", syncedInfo.Size())
		}
	}

	// second sync
	err = syncer.StoreRepo()
	if err != nil {
		t.Error(err)
	}
}
