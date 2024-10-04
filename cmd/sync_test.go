package cmd

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uyuni-project/minima/get"
	"github.com/uyuni-project/minima/storage"
)

const (
	testdataDir        = "testdata"
	invalidStoragefile = "invalid_storage.yaml"
	validHTTPReposFile = "valid_http_repos.yaml"
	validSCCReposFile  = "valid_scc_repos.yaml"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name      string
		inputFile string
		want      Config
		wantErr   bool
	}{
		{
			"Valid HTTP repos", validHTTPReposFile,
			Config{
				Storage: storage.StorageConfig{
					Type: "file",
					Path: "/srv/mirror",
				},
				HTTP: []get.HTTPRepoConfig{
					{
						URL:   "http://test/SLE-Product-SLES15-SP5-Pool/",
						Archs: []string{"x86_64", "aarch64", "s390x"},
					},
					{
						URL:   "http://test/SLE-Product-SLES15-SP5-Updates/",
						Archs: []string{"x86_64", "aarch64"},
					},
				},
			},
			false,
		},
		{
			"Valid SCC repos", validSCCReposFile,
			Config{
				Storage: storage.StorageConfig{
					Type: "file",
					Path: "/srv/mirror",
				},
				SCC: get.SCC{
					Username: "user",
					Password: "pass",
					Repositories: []get.SCCReposConfig{
						{
							Names: []string{"SLE-Manager-Tools15-Pool", "SLE-Manager-Tools15-Updates"},
							Archs: []string{"x86_64", "aarch64", "s390x"},
						},
						{
							Names: []string{"SLE-Product-SLES15-SP5-Pool", "SLE-Product-SLES15-SP5-Updates"},
							Archs: []string{"x86_64", "s390x"},
						},
					},
				},
			},
			false,
		},
		{
			"Invalid storage", invalidStoragefile,
			Config{
				Storage: storage.StorageConfig{
					Type: "memory",
					Path: "/srv/mirror",
				},
				HTTP: []get.HTTPRepoConfig{
					{
						URL:   "http://test/SLE-Product-SLES15-SP5-Pool/",
						Archs: []string{"x86_64", "aarch64", "s390x"},
					},
					{
						URL:   "http://test/SLE-Product-SLES15-SP5-Updates/",
						Archs: []string{"x86_64", "aarch64"},
					},
				},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := path.Join(testdataDir, tt.inputFile)
			bytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatal()
			}
			configString := string(bytes)

			got, err := parseConfig(configString)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			if !assert.ObjectsAreEqualValues(tt.want, got) {
				t.Errorf("Expected %v - got %v", tt.want, got)
			}
		})
	}
}
