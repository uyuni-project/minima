package storage

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  StorageConfig
		repoURL string
		want    Storage
		wantErr bool
	}{
		{
			"Invalid Storage type",
			StorageConfig{
				Type: "memory",
				Path: "/srv/mirror/",
			},
			"http://test/some-product/",
			nil,
			true,
		},
		{
			"File Storage",
			StorageConfig{
				Type: "file",
				Path: "/srv/mirror/",
			},
			"http://test/some-product/",
			&FileStorage{
				directory: "/srv/mirror/some-product",
			},
			false,
		},
		// TODO: S3 Storage
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoURL, err := url.Parse(tt.repoURL)
			if err != nil {
				t.Fatalf("Failed to parse repo URL %s: %v", tt.repoURL, err)
			}

			got, err := FromConfig(tt.config, repoURL)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			assert.EqualValues(t, tt.want, got)
		})
	}
}
