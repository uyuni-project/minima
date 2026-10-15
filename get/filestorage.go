package get

import (
	"crypto"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/uyuni-project/minima/util"
)

// FileStorage allows to store data in a local directory
type FileStorage struct {
	directory string
}

// NewFileStorage returns a new Storage given a local directory
func NewFileStorage(directory string) Storage {
	return &FileStorage{directory}
}

// NewReader returns a Reader for a file in a location, returns ErrFileNotFound
// if the requested path was not found at all
func (s *FileStorage) NewReader(filename string, location Location) (reader io.ReadCloser, err error) {
	var prefix string
	if location == Permanent {
		prefix = ""
	} else {
		prefix = "-in-progress"
	}
	fullPath, err := localPath(s.directory+prefix, filename)
	if err != nil {
		return
	}
	f, err := os.Open(fullPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// StoringMapper returns a mapper that will store read data to a temporary location specified by filename
func (s *FileStorage) StoringMapper(filename string, checksum string, hash crypto.Hash) util.ReaderMapper {
	return func(reader io.ReadCloser) (result io.ReadCloser, err error) {
		fullPath, err := localPath(s.directory+"-in-progress", filename)
		if err != nil {
			return
		}
		// attempt to create any missing directories in the full path
		err = os.MkdirAll(filepath.Dir(fullPath), os.ModePerm)
		if err != nil {
			return
		}

		file, err := os.Create(fullPath)
		if err != nil {
			return
		}

		result = util.NewTeeReadCloser(reader, util.NewChecksummingWriter(file, checksum, hash))
		return
	}
}

// Recycle will copy a file from the permanent to the temporary location
func (s *FileStorage) Recycle(filename string) (err error) {
	newPath, err := localPath(s.directory+"-in-progress", filename)
	if err != nil {
		return
	}
	err = os.MkdirAll(filepath.Dir(newPath), os.ModePerm)
	if err != nil {
		return
	}

	err = os.Link(filepath.Join(s.directory, filename), newPath)
	if errors.Is(err, fs.ErrExist) {
		// ignore, we are fine already
		return nil
	}
	return
}

// Commit will take care of moving downloaded metadata and packages in the target
// path, plus cleanup old or temporary files
func (s *FileStorage) Commit() error {
	oldDir := s.directory + "-old"
	tmpDir := s.directory + "-in-progress"

	// If in-progress contains actual packages, it is a candidate for being swapped with the target repo.
	// Otherwise, it's a situation where we only have metadata in x-in-progress.
	if hasPackages(tmpDir) {
		os.RemoveAll(oldDir)

		if err := os.Rename(s.directory, oldDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.Rename(tmpDir, s.directory); err != nil {
			return err
		}

		return os.RemoveAll(oldDir)
	}

	// Move all new files (likely just repodata) from -in-progress to the target
	if err := mergeDirs(tmpDir, s.directory); err != nil {
		return err
	}
	return os.RemoveAll(tmpDir)
}

// localPath joins filename to dir, rejecting names (which come from remote metadata) that would escape dir
func localPath(dir, filename string) (string, error) {
	if !filepath.IsLocal(filename) {
		return "", fmt.Errorf("refusing unsafe path %q", filename)
	}
	return filepath.Join(dir, filename), nil
}

func hasPackages(dir string) bool {
	found := false
	// We check for common package extensions used in Linux distros
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if _, check := packageExtensions[ext]; check {
				found = true
				return filepath.SkipAll // Stop walking as soon as one is found
			}
		}
		return nil
	})
	return found
}

// mergeDirs moves the contents of the repository at source path into the repository at target path
func mergeDirs(source, target string) error {
	// ensure target directory exists
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	dirs, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		from := filepath.Join(source, dir.Name())
		to := filepath.Join(target, dir.Name())
		// cleanup previous entries in the target to prevent errors
		_ = os.RemoveAll(to)

		err = os.Rename(from, to)
		if err != nil {
			return err
		}
	}

	return nil
}
