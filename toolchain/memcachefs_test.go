package toolchain

import "github.com/pawnkit/pawn-project/fsx"

// memCacheFS adds cache writes to fsx.Mem.
type memCacheFS struct{ *fsx.Mem }

func newMemCacheFS() memCacheFS {
	return memCacheFS{fsx.NewMem()}
}

func (m memCacheFS) MkdirAll(path string) error {
	m.AddDir(path)

	return nil
}

func (m memCacheFS) WriteFile(path string, content []byte) error {
	m.AddFile(path, content)

	return nil
}

func (m memCacheFS) RemoveAll(path string) error { return m.Mem.RemoveAll(path) }

func (m memCacheFS) Rename(oldPath, newPath string) error { return m.Mem.Rename(oldPath, newPath) }
