package toolchain

import (
	"os"

	"github.com/pawnkit/pawn-project/fsx"
)

// CacheFS adds the writes needed by the toolchain cache.
type CacheFS interface {
	fsx.FS
	MkdirAll(path string) error
	RemoveAll(path string) error
	Rename(oldPath, newPath string) error
	WriteFile(path string, content []byte) error // WriteFile stores content without making it executable.
}

// OSCacheFS implements [CacheFS] against the real filesystem.
type OSCacheFS struct {
	fsx.OS
}

func (OSCacheFS) MkdirAll(path string) error {
	return os.MkdirAll(path, 0o750)
}

func (OSCacheFS) WriteFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0o600)
}

func (OSCacheFS) RemoveAll(path string) error { return os.RemoveAll(path) }

func (OSCacheFS) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

// DefaultCacheDir returns the user cache directory for Pawn toolchains.
func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	return base + "/pawnkit/pawn-project/toolchains", nil
}
