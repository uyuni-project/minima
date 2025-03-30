package storage

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testdataDir   = "testdata/testrepo"
	inProgressDir = "testdata/testrepo-in-progress"
	testFile      = "test.txt"
)

func TestNewReader(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		filename  string
		location  Location
		wantErr   bool
	}{
		{"Permanent location", testdataDir, testFile, Permanent, false},
		{"Temporary location", testdataDir, testFile, Temporary, false},
		{"Not existing file", testdataDir, "does-not-exist.txt", Permanent, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewFileStorage(tt.directory)

			r, err := storage.NewReader(tt.filename, tt.location)
			assert.EqualValues(t, tt.wantErr, (err != nil))
			if r != nil {
				content, err := io.ReadAll(r)
				assert.Nil(t, err)
				assert.EqualValues(t, "Hello World", string(content))
			}
		})
	}
}
