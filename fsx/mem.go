package fsx

import (
	"io/fs"
	"maps"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Mem is a concurrent, in-memory [FS]. Paths use forward slashes.
type Mem struct {
	mu      sync.RWMutex
	entries map[string]*memEntry
}

type memEntry struct {
	name    string
	isDir   bool
	content []byte
	modTime time.Time
}

// NewMem returns an empty in-memory filesystem containing only the root
// directory "/".
func NewMem() *Mem {
	m := &Mem{entries: make(map[string]*memEntry)}
	m.entries["/"] = &memEntry{name: "/", isDir: true}

	return m
}

// AddFile creates path and any missing parent directories, then sets
// path's content.
func (m *Mem) AddFile(p string, content []byte) *Mem {
	m.mu.Lock()
	defer m.mu.Unlock()

	p = path.Clean(p)
	m.mkdirAllLocked(path.Dir(p))
	m.entries[p] = &memEntry{name: path.Base(p), content: content, modTime: time.Now()}

	return m
}

// AddDir creates path and any missing parent directories as directories.
func (m *Mem) AddDir(p string) *Mem {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mkdirAllLocked(path.Clean(p))

	return m
}

// RemoveAll removes a path and everything below it.
func (m *Mem) RemoveAll(p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p = path.Clean(p)
	if p == "/" || p == "." {
		return &fs.PathError{Op: "removeall", Path: p, Err: fs.ErrInvalid}
	}
	for candidate := range m.entries {
		if candidate == p || strings.HasPrefix(candidate, p+"/") {
			delete(m.entries, candidate)
		}
	}
	return nil
}

// Rename moves a path and everything below it.
func (m *Mem) Rename(oldPath, newPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldPath, newPath = path.Clean(oldPath), path.Clean(newPath)
	if oldPath == "/" || newPath == "/" || oldPath == "." || newPath == "." {
		return &fs.PathError{Op: "rename", Path: oldPath, Err: fs.ErrInvalid}
	}
	if _, ok := m.entries[oldPath]; !ok {
		return &fs.PathError{Op: "rename", Path: oldPath, Err: fs.ErrNotExist}
	}
	if _, ok := m.entries[newPath]; ok {
		return &fs.PathError{Op: "rename", Path: newPath, Err: fs.ErrExist}
	}
	m.mkdirAllLocked(path.Dir(newPath))
	moved := make(map[string]*memEntry)
	for candidate, entry := range m.entries {
		if candidate == oldPath || strings.HasPrefix(candidate, oldPath+"/") {
			newCandidate := newPath + strings.TrimPrefix(candidate, oldPath)
			copyEntry := *entry
			if candidate == oldPath {
				copyEntry.name = path.Base(newPath)
			}
			moved[newCandidate] = &copyEntry
			delete(m.entries, candidate)
		}
	}
	maps.Copy(m.entries, moved)
	return nil
}

func (m *Mem) mkdirAllLocked(p string) {
	if p == "/" || p == "." {
		m.entries["/"] = &memEntry{name: "/", isDir: true}

		return
	}

	if _, ok := m.entries[p]; ok {
		return
	}

	m.mkdirAllLocked(path.Dir(p))
	m.entries[p] = &memEntry{name: path.Base(p), isDir: true}
}

func (m *Mem) Stat(p string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p = path.Clean(p)

	e, ok := m.entries[p]
	if !ok {
		return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
	}

	return memFileInfo{e}, nil
}

func (m *Mem) ReadFile(p string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p = path.Clean(p)

	e, ok := m.entries[p]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: p, Err: fs.ErrNotExist}
	}

	if e.isDir {
		return nil, &fs.PathError{Op: "read", Path: p, Err: fs.ErrInvalid}
	}

	out := make([]byte, len(e.content))
	copy(out, e.content)

	return out, nil
}

func (m *Mem) ReadDir(p string) ([]fs.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p = path.Clean(p)

	dir, ok := m.entries[p]
	if !ok || !dir.isDir {
		return nil, &fs.PathError{Op: "readdir", Path: p, Err: fs.ErrNotExist}
	}

	var out []fs.DirEntry

	for candidate, e := range m.entries {
		if candidate == "/" || path.Dir(candidate) != p {
			continue
		}

		out = append(out, memDirEntry{e})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })

	return out, nil
}

type memFileInfo struct{ e *memEntry }

func (i memFileInfo) Name() string       { return i.e.name }
func (i memFileInfo) Size() int64        { return int64(len(i.e.content)) }
func (i memFileInfo) Mode() fs.FileMode  { return i.modeOf() }
func (i memFileInfo) ModTime() time.Time { return i.e.modTime }
func (i memFileInfo) IsDir() bool        { return i.e.isDir }
func (i memFileInfo) Sys() any           { return nil }

func (i memFileInfo) modeOf() fs.FileMode {
	if i.e.isDir {
		return fs.ModeDir | 0o755
	}

	return 0o644
}

type memDirEntry struct{ e *memEntry }

func (d memDirEntry) Name() string               { return d.e.name }
func (d memDirEntry) IsDir() bool                { return d.e.isDir }
func (d memDirEntry) Type() fs.FileMode          { return memFileInfo(d).Mode().Type() }
func (d memDirEntry) Info() (fs.FileInfo, error) { return memFileInfo(d), nil }

var (
	_ fs.DirEntry = memDirEntry{}
	_ fs.FileInfo = memFileInfo{}
)

// Paths returns all stored paths in sorted order.
func (m *Mem) Paths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, len(m.entries))
	for p := range m.entries {
		out = append(out, p)
	}

	slices.Sort(out)

	return out
}
