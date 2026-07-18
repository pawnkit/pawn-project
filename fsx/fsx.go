// Package fsx provides filesystem interfaces and test implementations.
package fsx

import "io/fs"

// FS is the read-only filesystem surface used by pawn-project.
type FS interface {
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]fs.DirEntry, error)
}

// Exists reports whether path can be statted.
func Exists(fsys FS, path string) bool {
	_, err := fsys.Stat(path)

	return err == nil
}

// IsDir reports whether path exists in fsys and is a directory.
func IsDir(fsys FS, path string) bool {
	info, err := fsys.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// IsFile reports whether path exists in fsys and is a regular file.
func IsFile(fsys FS, path string) bool {
	info, err := fsys.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
