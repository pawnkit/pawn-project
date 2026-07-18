// Package toolchain resolves, verifies, and caches Pawn compilers.
package toolchain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"unicode"

	"github.com/pawnkit/pawnkit-core/hash"
)

// Vendor identifies a compiler lineage.
type Vendor string

const (
	VendorPawnLang        Vendor = "pawn-lang"
	VendorOpenMultiplayer Vendor = "openmultiplayer"
	VendorOriginalPawn    Vendor = "original-pawn"
	VendorLocal           Vendor = "local"
)

// Info describes a resolved compiler toolchain.
type Info struct {
	Vendor           Vendor
	Version          string
	Path             string // absolute path to the compiler binary
	Checksum         string // "sha256:<hex>" of the binary at Path
	ArtifactChecksum string // checksum of the downloaded artifact
}

// Platform identifies a target platform for a downloadable artifact.
type Platform struct {
	OS   string // "linux", "windows", "darwin"
	Arch string // "x86_64", "arm64"
}

// String returns an "os-arch" platform identifier.
func (p Platform) String() string {
	return p.OS + "-" + p.Arch
}

// Clock provides Unix time for cache operations.
type Clock interface {
	Now() int64 // Unix seconds
}

// SystemClock calls the real wall clock.
type SystemClock struct{}

func (SystemClock) Now() int64 { return systemNow() }

// Downloader fetches a remote artifact by URL.
type Downloader interface {
	Download(ctx context.Context, url string) (io.ReadCloser, error)
}

// NoNetworkDownloader rejects every download with [ErrOffline].
type NoNetworkDownloader struct{}

func (NoNetworkDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, ErrOffline
}

var (
	// ErrOffline is returned when a resolution requires a download and
	// none is available (no Downloader configured, or Offline was set).
	ErrOffline = errors.New("toolchain: no network downloader configured (offline)")

	// ErrNotFound is returned when Resolve cannot locate a compiler by any
	// configured mode.
	ErrNotFound = errors.New("toolchain: no compiler could be resolved")

	// ErrChecksumMismatch is returned when a downloaded or locally
	// configured artifact's checksum does not match the expected value.
	ErrChecksumMismatch = errors.New("toolchain: checksum mismatch")

	// ErrInvalidCoordinate is returned for an unsupported vendor or unsafe
	// version string.
	ErrInvalidCoordinate = errors.New("toolchain: invalid toolchain coordinate")

	// ErrDownloadURLRequired is returned when a download is needed but the
	// caller did not provide an artifact URL.
	ErrDownloadURLRequired = errors.New("toolchain: download URL required")
)

// Resolver resolves, caches, and lists compiler toolchains.
type Resolver struct {
	fsys       CacheFS
	cacheDir   string
	downloader Downloader
	clock      Clock
	cacheMu    sync.Mutex
}

// NewResolver builds a Resolver. A nil downloader disables network access.
func NewResolver(fsys CacheFS, cacheDir string, downloader Downloader, clock Clock) *Resolver {
	if downloader == nil {
		downloader = NoNetworkDownloader{}
	}

	if clock == nil {
		clock = SystemClock{}
	}

	return &Resolver{fsys: fsys, cacheDir: cacheDir, downloader: downloader, clock: clock}
}

// ResolveOptions configures one Resolve call.
type ResolveOptions struct {
	// LocalPath takes precedence and never uses the network.
	LocalPath string

	// Vendor and Version identify a pinned toolchain.
	Vendor  Vendor
	Version string

	// DownloadURL is the HTTPS artifact URL used when the requested compiler
	// is not cached. It is ignored for local paths and cache hits.
	DownloadURL string

	// ExpectedChecksum verifies the local or downloaded artifact when set.
	ExpectedChecksum string

	// Offline restricts Resolve to local and cached compilers.
	Offline bool
}

// Resolve checks a local path, the cache, then the configured download.
// Cached versions are never updated implicitly.
func (r *Resolver) Resolve(ctx context.Context, opts ResolveOptions) (Info, error) {
	if opts.LocalPath != "" {
		return r.resolveLocal(opts)
	}
	if err := validateCoordinate(opts.Vendor, opts.Version); err != nil {
		return Info{}, err
	}

	r.cacheMu.Lock()
	cached, ok, cacheErr := r.lookupCache(opts.Vendor, opts.Version)
	if cacheErr == nil && ok {
		cacheErr = r.verifyCached(cached, opts.ExpectedChecksum)
	}
	r.cacheMu.Unlock()
	if cacheErr != nil {
		return Info{}, cacheErr
	} else if ok {
		return cached, nil
	}

	if opts.Offline {
		return Info{}, fmt.Errorf("%w: %s/%s not cached and Offline is set", ErrOffline, opts.Vendor, opts.Version)
	}

	return r.download(ctx, opts)
}

func (r *Resolver) resolveLocal(opts ResolveOptions) (Info, error) {
	info, err := r.fsys.Stat(opts.LocalPath)
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: local compiler %q: %w", opts.LocalPath, err)
	}

	if info.IsDir() {
		return Info{}, fmt.Errorf("toolchain: local compiler %q is a directory", opts.LocalPath)
	}

	content, err := r.fsys.ReadFile(opts.LocalPath)
	if err != nil {
		return Info{}, fmt.Errorf("toolchain: reading local compiler %q: %w", opts.LocalPath, err)
	}

	sum := hash.Content(content)

	if opts.ExpectedChecksum != "" && opts.ExpectedChecksum != sum {
		return Info{}, fmt.Errorf("%w: local compiler %q: got %s, want %s",
			ErrChecksumMismatch, opts.LocalPath, sum, opts.ExpectedChecksum)
	}

	vendor := opts.Vendor
	if vendor == "" {
		vendor = VendorLocal
	}

	return Info{Vendor: vendor, Version: opts.Version, Path: opts.LocalPath, Checksum: sum}, nil
}

// List returns every toolchain currently in the cache.
func (r *Resolver) List() ([]Info, error) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	entries, err := r.fsys.ReadDir(r.cacheDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("toolchain: listing cache %q: %w", r.cacheDir, err)
	}

	var out []Info

	for _, vendorEntry := range entries {
		if !vendorEntry.IsDir() {
			continue
		}

		versionEntries, err := r.fsys.ReadDir(r.cacheDir + "/" + vendorEntry.Name())
		if err != nil {
			return nil, fmt.Errorf("toolchain: listing cache %q: %w", r.cacheDir, err)
		}

		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}

			info, ok, err := r.readCacheEntry(vendorEntry.Name(), versionEntry.Name())
			if err != nil {
				return nil, err
			}

			if ok {
				out = append(out, info)
			}
		}
	}

	return out, nil
}

// Update downloads and replaces a cached toolchain. Offline is ignored.
func (r *Resolver) Update(ctx context.Context, opts ResolveOptions) (Info, error) {
	if opts.LocalPath != "" {
		return r.resolveLocal(opts)
	}
	if err := validateCoordinate(opts.Vendor, opts.Version); err != nil {
		return Info{}, err
	}

	return r.download(ctx, opts)
}

func (r *Resolver) download(ctx context.Context, opts ResolveOptions) (Info, error) {
	if opts.DownloadURL == "" {
		return Info{}, fmt.Errorf("%w for %s/%s", ErrDownloadURLRequired, opts.Vendor, opts.Version)
	}

	rc, err := r.downloader.Download(ctx, opts.DownloadURL)
	if err != nil {
		return Info{}, err
	}

	defer func() { _ = rc.Close() }()

	return r.storeFromDownload(opts, rc)
}

func validateCoordinate(vendor Vendor, version string) error {
	switch vendor {
	case VendorPawnLang, VendorOpenMultiplayer, VendorOriginalPawn:
	default:
		return fmt.Errorf("%w: unsupported vendor %q", ErrInvalidCoordinate, vendor)
	}

	if version == "" || version == "." || version == ".." || strings.ContainsAny(version, `/\\`) {
		return fmt.Errorf("%w: unsafe version %q", ErrInvalidCoordinate, version)
	}
	for _, r := range version {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: unsafe version %q", ErrInvalidCoordinate, version)
		}
	}

	return nil
}
