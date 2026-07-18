package fsx

import (
	"io/fs"
	"os"
)

// OS implements [FS] against the real filesystem. The zero value is ready
// to use.
type OS struct{}

func (OS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (OS) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // reading a caller-supplied path is this abstraction's entire purpose.
}

func (OS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}
