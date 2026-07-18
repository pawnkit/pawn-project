package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/pawnkit/pawnkit-core/hash"
)

type fakeDownloader struct {
	calls int
	data  []byte
	err   error
}

func (f *fakeDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}

	return io.NopCloser(bytes.NewReader(f.data)), nil
}

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q): %v", name, err)
		}

		if _, err := w.Write(content); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}

	return buf.Bytes()
}

func TestResolveLocal_Success(t *testing.T) {
	fsys := newMemCacheFS()
	fsys.AddFile("/opt/pawncc", []byte("binary-content"))

	r := NewResolver(fsys, "/cache", nil, nil)

	info, err := r.Resolve(context.Background(), ResolveOptions{LocalPath: "/opt/pawncc", Version: "3.10.11"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if info.Path != "/opt/pawncc" || info.Vendor != VendorLocal {
		t.Errorf("info = %+v", info)
	}

	if info.Checksum != hash.Content([]byte("binary-content")) {
		t.Errorf("Checksum = %q", info.Checksum)
	}
}

func TestResolveLocal_ChecksumMismatch(t *testing.T) {
	fsys := newMemCacheFS()
	fsys.AddFile("/opt/pawncc", []byte("binary-content"))

	r := NewResolver(fsys, "/cache", nil, nil)

	_, err := r.Resolve(context.Background(), ResolveOptions{
		LocalPath:        "/opt/pawncc",
		ExpectedChecksum: "sha256:deadbeef",
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
}

func TestResolveLocal_MissingFile(t *testing.T) {
	fsys := newMemCacheFS()

	r := NewResolver(fsys, "/cache", nil, nil)

	if _, err := r.Resolve(context.Background(), ResolveOptions{LocalPath: "/opt/pawncc"}); err == nil {
		t.Fatal("expected error for missing local compiler")
	}
}

func TestResolve_NotCachedNoNetwork(t *testing.T) {
	fsys := newMemCacheFS()

	r := NewResolver(fsys, "/cache", nil, nil)

	_, err := r.Resolve(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc"})
	if !errors.Is(err, ErrOffline) {
		t.Fatalf("err = %v, want ErrOffline", err)
	}
}

func TestResolve_RejectsUnsafeCoordinate(t *testing.T) {
	fSys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("compiler")}
	r := NewResolver(fSys, "/cache", dl, nil)

	_, err := r.Resolve(context.Background(), ResolveOptions{
		Vendor:      VendorOpenMultiplayer,
		Version:     "../../escape",
		DownloadURL: "https://example.test/pawncc",
	})
	if !errors.Is(err, ErrInvalidCoordinate) {
		t.Fatalf("error = %v, want ErrInvalidCoordinate", err)
	}
	if dl.calls != 0 {
		t.Fatalf("downloader calls = %d, want 0", dl.calls)
	}
}

func TestResolve_DownloadsAndCaches(t *testing.T) {
	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("compiler")}
	r := NewResolver(fsys, "/cache", dl, nil)
	info, err := r.Resolve(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "1", DownloadURL: "https://example.test/pawncc"})
	if err != nil {
		t.Fatal(err)
	}
	if dl.calls != 1 || info.Checksum != hash.Content([]byte("compiler")) {
		t.Fatalf("calls=%d info=%+v", dl.calls, info)
	}
}

func TestResolve_RejectsCorruptCache(t *testing.T) {
	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("compiler")}
	r := NewResolver(fsys, "/cache", dl, nil)
	info, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "1", DownloadURL: "https://example.test/pawncc"})
	if err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(info.Path, []byte("corrupt")); err != nil {
		t.Fatal(err)
	}
	_, err = r.Resolve(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "1", DownloadURL: "https://example.test/pawncc"})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolve_OfflineReturnsErrOffline(t *testing.T) {
	fsys := newMemCacheFS()

	r := NewResolver(fsys, "/cache", nil, nil)

	_, err := r.Resolve(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc", Offline: true})
	if !errors.Is(err, ErrOffline) {
		t.Fatalf("err = %v, want ErrOffline", err)
	}
}

func TestUpdate_NoDownloaderConfigured(t *testing.T) {
	fsys := newMemCacheFS()

	r := NewResolver(fsys, "/cache", nil, nil)

	_, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc"})
	if !errors.Is(err, ErrOffline) {
		t.Fatalf("err = %v, want ErrOffline", err)
	}
}

func TestUpdate_RawBinaryStoredAndCached(t *testing.T) {
	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("fake-compiler-bytes")}
	r := NewResolver(fsys, "/cache", dl, nil)

	opts := ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc"}

	info, err := r.Update(context.Background(), opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if dl.calls != 1 {
		t.Fatalf("downloader calls = %d, want 1", dl.calls)
	}

	if info.Checksum != hash.Content([]byte("fake-compiler-bytes")) {
		t.Errorf("Checksum = %q", info.Checksum)
	}

	// Resolve now finds it from cache without calling the downloader again
	// (never update silently during a deterministic build).
	resolved, err := r.Resolve(context.Background(), opts)
	if err != nil {
		t.Fatalf("Resolve after cache: %v", err)
	}

	if resolved.Path != info.Path {
		t.Errorf("resolved.Path = %q, want %q", resolved.Path, info.Path)
	}

	if dl.calls != 1 {
		t.Fatalf("downloader calls after Resolve = %d, want still 1 (cache hit)", dl.calls)
	}
}

func TestUpdate_ChecksumMismatchNotStored(t *testing.T) {
	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("fake-compiler-bytes")}
	r := NewResolver(fsys, "/cache", dl, nil)

	opts := ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc", ExpectedChecksum: "sha256:deadbeef"}

	if _, err := r.Update(context.Background(), opts); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}

	if _, ok, _ := r.lookupCache(opts.Vendor, opts.Version); ok {
		t.Error("cache should not contain a mismatched download")
	}
}

func TestUpdate_ZipArchiveExtracted(t *testing.T) {
	zipBytes := buildZip(t, map[string][]byte{
		"pawncc":      []byte("#!/bin/sh\necho fake"),
		"README.txt":  []byte("readme"),
		"lib/std.pwn": []byte("stock x() {}"),
	})

	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: zipBytes}
	r := NewResolver(fsys, "/cache", dl, nil)

	info, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if info.Path == "" {
		t.Fatal("expected a resolved path")
	}

	content, err := fsys.ReadFile(info.Path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", info.Path, err)
	}

	if string(content) != "#!/bin/sh\necho fake" {
		t.Errorf("content = %q, want the pawncc entry to be selected as the binary", content)
	}
}

func TestUpdate_NestedCompilerPath(t *testing.T) {
	zipBytes := buildZip(t, map[string][]byte{"bin/pawncc": []byte("compiler")})
	fSys := newMemCacheFS()
	r := NewResolver(fSys, "/cache", &fakeDownloader{data: zipBytes}, nil)

	info, err := r.Update(context.Background(), ResolveOptions{
		Vendor: VendorOpenMultiplayer, Version: "3.10.11",
		DownloadURL: "https://example.test/pawncc.zip",
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != "/cache/openmultiplayer/3.10.11/bin/pawncc" {
		t.Fatalf("path = %q", info.Path)
	}
}

func TestResolve_RejectsEscapingCachedBinary(t *testing.T) {
	fSys := newMemCacheFS()
	fSys.AddFile("/cache/openmultiplayer/1/toolchain.json", []byte(`{"vendor":"openmultiplayer","version":"1","checksum":"sha256:test","binary":"../../../outside"}`))
	r := NewResolver(fSys, "/cache", nil, nil)

	_, err := r.Resolve(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "1"})
	if err == nil {
		t.Fatal("unsafe cached binary path accepted")
	}
}

func TestUpdate_ZipTraversalRejected(t *testing.T) {
	malicious := buildZip(t, map[string][]byte{"../../evil.sh": []byte("rm -rf /")})

	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: malicious}
	r := NewResolver(fsys, "/cache", dl, nil)

	_, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "evil", DownloadURL: "https://example.test/pawncc"})
	if !errors.Is(err, ErrArchiveTraversal) {
		t.Fatalf("err = %v, want ErrArchiveTraversal", err)
	}

	for _, p := range fsys.Paths() {
		if p == "/evil.sh" || p == "/cache/evil.sh" {
			t.Fatalf("traversal entry escaped extraction root: %s", p)
		}
	}
}

func TestUpdate_CorruptArchive(t *testing.T) {
	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("PK\x03\x04not-actually-a-valid-zip-body-at-all")}
	r := NewResolver(fsys, "/cache", dl, nil)

	_, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "corrupt", DownloadURL: "https://example.test/pawncc"})
	if err == nil {
		t.Fatal("expected an error for a corrupt archive, not success")
	}
}

func TestUpdate_LocalPathBypassesDownloader(t *testing.T) {
	fsys := newMemCacheFS()
	fsys.AddFile("/opt/pawncc", []byte("local"))

	dl := &fakeDownloader{data: []byte("should not be used")}
	r := NewResolver(fsys, "/cache", dl, nil)

	if _, err := r.Update(context.Background(), ResolveOptions{LocalPath: "/opt/pawncc"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if dl.calls != 0 {
		t.Errorf("downloader calls = %d, want 0", dl.calls)
	}
}

func TestList_EmptyCache(t *testing.T) {
	fsys := newMemCacheFS()
	r := NewResolver(fsys, "/cache", nil, nil)

	list, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 0 {
		t.Errorf("list = %v, want empty", list)
	}
}

func TestList_AfterUpdates(t *testing.T) {
	fsys := newMemCacheFS()
	dl := &fakeDownloader{data: []byte("v1")}
	r := NewResolver(fsys, "/cache", dl, nil)

	if _, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorOpenMultiplayer, Version: "3.10.11", DownloadURL: "https://example.test/pawncc"}); err != nil {
		t.Fatalf("Update 1: %v", err)
	}

	dl.data = []byte("v2-different")

	if _, err := r.Update(context.Background(), ResolveOptions{Vendor: VendorPawnLang, Version: "3.10.10", DownloadURL: "https://example.test/pawncc"}); err != nil {
		t.Fatalf("Update 2: %v", err)
	}

	list, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("list = %+v, want 2 entries", list)
	}
}

func TestPlatformString(t *testing.T) {
	if got := (Platform{OS: "linux", Arch: "x86_64"}).String(); got != "linux-x86_64" {
		t.Errorf("String = %q", got)
	}
}
