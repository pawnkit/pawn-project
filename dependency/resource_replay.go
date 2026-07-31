package dependency

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// ReplayDownloader keeps bounded downloads for reuse during one operation.
type ReplayDownloader struct {
	base   ResourceDownloader
	dir    string
	mu     sync.Mutex
	files  map[string]string
	closed bool
}

// NewReplayDownloader creates a temporary download replay store.
func NewReplayDownloader(base ResourceDownloader) (*ReplayDownloader, error) {
	if base == nil {
		return nil, errors.New("dependency: resource downloader is required")
	}
	dir, err := os.MkdirTemp("", "pawnkit-resource-downloads-")
	if err != nil {
		return nil, fmt.Errorf("dependency: creating resource download store: %w", err)
	}
	return &ReplayDownloader{
		base:  base,
		dir:   dir,
		files: make(map[string]string),
	}, nil
}

// Download returns a fresh reader and downloads each URL at most once.
func (d *ReplayDownloader) Download(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil, errors.New("dependency: resource download store is closed")
	}
	if filename, ok := d.files[rawURL]; ok {
		return os.Open(filename) //nolint:gosec // Paths are created inside the private store.
	}

	reader, err := d.base.Download(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()

	file, err := os.CreateTemp(d.dir, "asset-")
	if err != nil {
		return nil, fmt.Errorf("dependency: creating resource download: %w", err)
	}
	filename := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(filename)
		}
	}()

	written, err := io.Copy(file, io.LimitReader(reader, maxResourceDownload+1))
	if err != nil {
		return nil, fmt.Errorf("dependency: caching resource download: %w", err)
	}
	if written > maxResourceDownload {
		return nil, fmt.Errorf("dependency: resource download exceeds %d bytes", maxResourceDownload)
	}
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("dependency: syncing resource download: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("dependency: closing resource download: %w", err)
	}

	d.files[rawURL] = filename
	keep = true
	return os.Open(filepath.Clean(filename))
}

// Close removes the temporary replay store.
func (d *ReplayDownloader) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}
	d.closed = true
	if err := os.RemoveAll(d.dir); err != nil {
		return fmt.Errorf("dependency: removing resource download store: %w", err)
	}
	return nil
}
