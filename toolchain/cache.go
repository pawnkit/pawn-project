package toolchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/pawnkit/pawn-project/pathutil"
	"github.com/pawnkit/pawnkit-core/hash"
)

const (
	metadataFileName = "toolchain.json"
	binaryFileName   = "pawncc"

	// maxDownloadBytes bounds downloaded artifacts.
	maxDownloadBytes = 512 * 1024 * 1024
)

type cacheMetadata struct {
	Vendor           Vendor `json:"vendor"`
	Version          string `json:"version"`
	Checksum         string `json:"checksum"`
	ArtifactChecksum string `json:"artifactChecksum,omitempty"`
	Binary           string `json:"binary"`
}

func (r *Resolver) cacheEntryDir(vendor Vendor, version string) string {
	return fmt.Sprintf("%s/%s/%s", r.cacheDir, vendor, version)
}

func (r *Resolver) lookupCache(vendor Vendor, version string) (Info, bool, error) {
	if vendor == "" || version == "" {
		return Info{}, false, nil
	}

	dir := r.cacheEntryDir(vendor, version)

	meta, ok, err := r.readMetadata(dir)
	if err != nil || !ok {
		return Info{}, ok, err
	}

	if meta.Vendor != vendor || meta.Version != version {
		return Info{}, false, fmt.Errorf("toolchain: cache metadata does not match %s/%s", vendor, version)
	}

	info, err := cacheInfo(dir, meta)
	return info, err == nil, err
}

func (r *Resolver) readMetadata(dir string) (cacheMetadata, bool, error) {
	metaPath := dir + "/" + metadataFileName

	if _, err := r.fsys.Stat(metaPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cacheMetadata{}, false, nil
		}

		return cacheMetadata{}, false, fmt.Errorf("toolchain: checking %q: %w", metaPath, err)
	}

	content, err := r.fsys.ReadFile(metaPath)
	if err != nil {
		return cacheMetadata{}, false, fmt.Errorf("toolchain: reading %q: %w", metaPath, err)
	}

	var meta cacheMetadata
	if err := json.Unmarshal(content, &meta); err != nil {
		return cacheMetadata{}, false, fmt.Errorf("toolchain: parsing %q: %w", metaPath, err)
	}

	return meta, true, nil
}

func (r *Resolver) readCacheEntry(vendorDir, versionDir string) (Info, bool, error) {
	dir := r.cacheDir + "/" + vendorDir + "/" + versionDir

	meta, ok, err := r.readMetadata(dir)
	if err != nil || !ok {
		return Info{}, ok, err
	}

	if string(meta.Vendor) != vendorDir || meta.Version != versionDir {
		return Info{}, false, fmt.Errorf("toolchain: cache metadata does not match directory %s/%s", vendorDir, versionDir)
	}

	info, err := cacheInfo(dir, meta)
	return info, err == nil, err
}

func cacheInfo(dir string, meta cacheMetadata) (Info, error) {
	if meta.Binary == "" || meta.Checksum == "" {
		return Info{}, errors.New("toolchain: incomplete cache metadata")
	}
	binaryPath, err := pathutil.SafeJoin(dir, meta.Binary)
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: unsafe cached binary path %q: %w", meta.Binary, err)
	}
	return Info{Vendor: meta.Vendor, Version: meta.Version, Path: binaryPath, Checksum: meta.Checksum, ArtifactChecksum: meta.ArtifactChecksum}, nil
}

func (r *Resolver) verifyCached(info Info, expected string) error {
	content, err := r.fsys.ReadFile(info.Path)
	if err != nil {
		return fmt.Errorf("toolchain: reading cached compiler %q: %w", info.Path, err)
	}
	actual := hash.Content(content)
	if actual != info.Checksum || expected != "" && expected != info.ArtifactChecksum && expected != actual {
		return fmt.Errorf("%w: cached compiler %q", ErrChecksumMismatch, info.Path)
	}
	return nil
}

// storeFromDownload verifies and stages a downloaded toolchain.
func (r *Resolver) storeFromDownload(opts ResolveOptions, rc io.Reader) (Info, error) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	limited := io.LimitReader(rc, maxDownloadBytes+1)

	data, err := io.ReadAll(limited)
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: reading download: %w", err)
	}

	if len(data) > maxDownloadBytes {
		return Info{}, fmt.Errorf("toolchain: download exceeds %d byte limit", maxDownloadBytes)
	}

	sum := hash.Content(data)

	if opts.ExpectedChecksum != "" && opts.ExpectedChecksum != sum {
		return Info{}, fmt.Errorf("%w: got %s, want %s", ErrChecksumMismatch, sum, opts.ExpectedChecksum)
	}

	dir := r.cacheEntryDir(opts.Vendor, opts.Version)
	stagingDir := dir + ".tmp"
	backupDir := dir + ".old"
	if err := r.fsys.RemoveAll(stagingDir); err != nil {
		return Info{}, fmt.Errorf("toolchain: clearing staging dir %q: %w", stagingDir, err)
	}
	defer func() { _ = r.fsys.RemoveAll(stagingDir) }()
	if err := r.fsys.MkdirAll(stagingDir); err != nil {
		return Info{}, fmt.Errorf("toolchain: creating cache dir %q: %w", stagingDir, err)
	}

	binary := binaryFileName

	if isZip(data) {
		extracted, err := extractZip(r.fsys, stagingDir, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return Info{}, err
		}

		binary = extracted
	} else if err := r.fsys.WriteFile(stagingDir+"/"+binary, data); err != nil {
		return Info{}, fmt.Errorf("toolchain: writing %q: %w", stagingDir+"/"+binary, err)
	}

	binaryContent, err := r.fsys.ReadFile(stagingDir + "/" + binary)
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: reading cached binary: %w", err)
	}
	binarySum := hash.Content(binaryContent)
	meta := cacheMetadata{Vendor: opts.Vendor, Version: opts.Version, Checksum: binarySum, ArtifactChecksum: sum, Binary: binary}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: encoding cache metadata: %w", err)
	}

	if err := r.fsys.WriteFile(stagingDir+"/"+metadataFileName, metaJSON); err != nil {
		return Info{}, fmt.Errorf("toolchain: writing cache metadata: %w", err)
	}

	if err := r.commitCacheEntry(dir, stagingDir, backupDir); err != nil {
		return Info{}, err
	}

	info, err := cacheInfo(dir, meta)
	if err != nil {
		return Info{}, err
	}
	return info, nil
}

func (r *Resolver) commitCacheEntry(dir, stagingDir, backupDir string) error {
	_ = r.fsys.RemoveAll(backupDir)
	_, statErr := r.fsys.Stat(dir)
	hadExisting := statErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("toolchain: checking cache dir %q: %w", dir, statErr)
	}
	if hadExisting {
		if err := r.fsys.Rename(dir, backupDir); err != nil {
			return fmt.Errorf("toolchain: preserving cache entry %q: %w", dir, err)
		}
	}
	if err := r.fsys.Rename(stagingDir, dir); err != nil {
		if hadExisting {
			_ = r.fsys.Rename(backupDir, dir)
		}
		return fmt.Errorf("toolchain: committing cache entry %q: %w", dir, err)
	}
	if err := r.fsys.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("toolchain: removing old cache entry %q: %w", backupDir, err)
	}
	return nil
}
