package dependency

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/pawnkit/pawnkit-core/hash"

	"github.com/pawnkit/pawn-project/lockfile"
)

type resourceDownloader struct {
	data []byte
}

func (d resourceDownloader) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(d.data)), nil
}

func TestFetchResourceRaw(t *testing.T) {
	data := []byte("plugin")
	resource := testResource("file", data, lockfile.ResolvedResourceFile{
		Source: "plugin.so", Destination: "plugins/plugin.so",
		Size: int64(len(data)), Checksum: hash.Content(data),
	})

	payload, err := FetchResource(context.Background(), resourceDownloader{data}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(payload["plugins/plugin.so"]); got != "plugin" {
		t.Fatalf("payload = %q", got)
	}
}

func TestFetchResourceZip(t *testing.T) {
	content := []byte("plugin")
	archive := zipResource(t, map[string][]byte{
		"release/plugin.so": content,
		"README.md":         []byte("ignored"),
	})
	resource := testResource("zip", archive, lockfile.ResolvedResourceFile{
		Source: "release/plugin.so", Destination: "plugins/plugin.so",
		Size: int64(len(content)), Checksum: hash.Content(content),
	})

	payload, err := FetchResource(context.Background(), resourceDownloader{archive}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(payload["plugins/plugin.so"]); got != "plugin" {
		t.Fatalf("payload = %q", got)
	}
}

func TestFetchResourceTarGzip(t *testing.T) {
	content := []byte("component")
	archive := tarResource(t, map[string][]byte{"components/test.so": content})
	resource := testResource("tar.gz", archive, lockfile.ResolvedResourceFile{
		Source: "components/test.so", Destination: "components/test.so",
		Size: int64(len(content)), Checksum: hash.Content(content),
	})

	payload, err := FetchResource(context.Background(), resourceDownloader{archive}, resource)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(payload["components/test.so"]); got != "component" {
		t.Fatalf("payload = %q", got)
	}
}

func TestFetchResourceRejectsArchiveChecksumMismatch(t *testing.T) {
	data := []byte("plugin")
	resource := testResource("file", data, lockfile.ResolvedResourceFile{
		Source: "plugin.so", Destination: "plugins/plugin.so",
		Size: int64(len(data)), Checksum: hash.Content(data),
	})
	resource.Checksum = "sha256:" + strings.Repeat("0", 64)

	if _, err := FetchResource(context.Background(), resourceDownloader{data}, resource); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
}

func TestFetchResourceRejectsUnsafeZipEntry(t *testing.T) {
	archive := zipResource(t, map[string][]byte{"../plugin.so": []byte("plugin")})
	resource := testResource("zip", archive, lockfile.ResolvedResourceFile{
		Source: "../plugin.so", Destination: "plugins/plugin.so",
		Size: 6, Checksum: hash.Content([]byte("plugin")),
	})

	if _, err := FetchResource(context.Background(), resourceDownloader{archive}, resource); err == nil {
		t.Fatal("traversal entry accepted")
	}
}

func testResource(
	format string,
	archive []byte,
	files ...lockfile.ResolvedResourceFile,
) lockfile.ResolvedResource {
	return lockfile.ResolvedResource{
		Package: "plugin://owner/package", Resource: "plugin",
		Target: "linux-amd64", URL: "https://example.com/resource",
		Size: int64(len(archive)), Checksum: hash.Content(archive),
		Archive: format, Files: files,
	}
}

func zipResource(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func tarResource(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	for name, content := range files {
		if err := writer.WriteHeader(&tar.Header{
			Name: name, Mode: 0o600, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
