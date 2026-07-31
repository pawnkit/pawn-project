package dependency

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawnkit-core/hash"
)

func TestResourceRestorerInstallsExactTargetAsOnePayload(t *testing.T) {
	root := t.TempDir()
	linuxPlugin := []byte("linux plugin")
	windowsPlugin := []byte("windows plugin")
	include := []byte("include")
	lock := &lockfile.Lock{Resources: []lockfile.ResolvedResource{
		resourceRecord("pkg/plugin", "plugin", "windows-amd64", "https://example.com/windows", "plugins/plugin.dll", windowsPlugin),
		resourceRecord("pkg/include", "include", "linux-amd64", "https://example.com/include", "include/pkg.inc", include),
		resourceRecord("pkg/plugin", "plugin", "linux-amd64", "https://example.com/linux", "plugins/plugin.so", linuxPlugin),
	}}
	downloader := &mapResourceDownloader{content: map[string][]byte{
		"https://example.com/linux":   linuxPlugin,
		"https://example.com/include": include,
	}}

	results, err := NewResourceRestorer(OSResourceFS{}, downloader).
		Restore(context.Background(), root, "linux-amd64", lock)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	assertResourceTestFile(t, root, "plugins/plugin.so", "linux plugin")
	assertResourceTestFile(t, root, "include/pkg.inc", "include")
	if len(downloader.urls) != 2 {
		t.Fatalf("downloads = %v", downloader.urls)
	}
}

func TestResourceRestorerRequiresExactTarget(t *testing.T) {
	data := []byte("plugin")
	lock := &lockfile.Lock{Resources: []lockfile.ResolvedResource{
		resourceRecord("pkg/plugin", "plugin", "linux-amd64", "https://example.com/plugin", "plugin.so", data),
	}}
	downloader := &mapResourceDownloader{}

	_, err := NewResourceRestorer(OSResourceFS{}, downloader).
		Restore(context.Background(), t.TempDir(), "windows-amd64", lock)
	if err == nil || !strings.Contains(err.Error(), "no record for target") {
		t.Fatalf("Restore error = %v", err)
	}
	if len(downloader.urls) != 0 {
		t.Fatalf("downloads = %v", downloader.urls)
	}
}

func TestResourceRestorerRejectsDestinationCollisionsBeforeDownload(t *testing.T) {
	first := []byte("first")
	second := []byte("second")
	lock := &lockfile.Lock{Resources: []lockfile.ResolvedResource{
		resourceRecord("pkg/first", "first", "linux-amd64", "https://example.com/first", "plugins/a.so", first),
		resourceRecord("pkg/second", "second", "linux-amd64", "https://example.com/second", "PLUGINS/A.SO", second),
	}}
	downloader := &mapResourceDownloader{}

	_, err := NewResourceRestorer(OSResourceFS{}, downloader).
		Restore(context.Background(), t.TempDir(), "linux-amd64", lock)
	if err == nil || !strings.Contains(err.Error(), "shared") {
		t.Fatalf("Restore error = %v", err)
	}
	if len(downloader.urls) != 0 {
		t.Fatalf("downloads = %v", downloader.urls)
	}
}

type mapResourceDownloader struct {
	content map[string][]byte
	urls    []string
}

func (d *mapResourceDownloader) Download(_ context.Context, url string) (io.ReadCloser, error) {
	d.urls = append(d.urls, url)
	return io.NopCloser(bytes.NewReader(d.content[url])), nil
}

func resourceRecord(
	pkg, name, target, url, destination string,
	data []byte,
) lockfile.ResolvedResource {
	return lockfile.ResolvedResource{
		Package: pkg, Resource: name, Target: target, URL: url,
		Size: int64(len(data)), Checksum: hash.Content(data), Archive: "file",
		Files: []lockfile.ResolvedResourceFile{{
			Source: destination, Destination: destination,
			Size: int64(len(data)), Checksum: hash.Content(data),
		}},
	}
}
