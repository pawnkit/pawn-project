package dependency

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/manifest"
)

func TestResolveResourceRecordMapsZIPFiles(t *testing.T) {
	archive := zipResource(t, map[string][]byte{
		"pawno/include/streamer.inc": []byte("include"),
		"pawno/include/extra.inc":    []byte("extra"),
		"plugins/streamer.so":        []byte("plugin"),
		"README.md":                  []byte("readme"),
		"source/main.cpp":            []byte("ignored"),
	})
	resource := manifest.Resource{
		Name: `^streamer-(.*)\.zip$`, Platform: "linux", Archive: true,
		Includes: []string{"pawno/include"},
		Plugins:  []string{"plugins/streamer.so"},
		Files:    map[string]string{"README.md": "docs/"},
	}
	asset := ReleaseAsset{
		Name: "streamer-v1.zip", URL: "https://example.com/streamer.zip",
		Size: int64(len(archive)),
	}

	record, err := ResolveResourceRecord(
		context.Background(),
		resourceDownloader{archive},
		"plugin://owner/streamer",
		"linux-amd64",
		resource,
		asset,
	)
	if err != nil {
		t.Fatalf("ResolveResourceRecord: %v", err)
	}
	if record.Archive != "zip" || len(record.Files) != 4 {
		t.Fatalf("record = %+v", record)
	}
	destinations := make([]string, len(record.Files))
	for i, file := range record.Files {
		destinations[i] = file.Destination
	}
	includeDir := resourceIncludeDirectory("plugin://owner/streamer", resource.Name)
	want := []string{
		includeDir + "/extra.inc",
		includeDir + "/streamer.inc",
		"docs/README.md",
		"plugins/streamer.so",
	}
	if !slices.Equal(destinations, want) {
		t.Fatalf("destinations = %v, want %v", destinations, want)
	}

	payload, err := FetchResource(context.Background(), resourceDownloader{archive}, record)
	if err != nil {
		t.Fatalf("FetchResource: %v", err)
	}
	if len(payload) != len(want) {
		t.Fatalf("payload files = %d, want %d", len(payload), len(want))
	}
}

func TestResolveResourceRecordMapsTarAndComponent(t *testing.T) {
	archive := tarResource(t, map[string][]byte{
		"components/sample.so": []byte("component"),
	})
	resource := manifest.Resource{
		Name: `sample\.tar\.gz$`, Platform: "linux", Archive: true,
		Plugins: []string{"components/sample.so"},
	}
	record, err := ResolveResourceRecord(
		context.Background(),
		resourceDownloader{archive},
		"component://owner/sample",
		"linux-amd64",
		resource,
		ReleaseAsset{
			Name: "sample.tar.gz", URL: "https://example.com/sample.tar.gz",
			Size: int64(len(archive)),
		},
	)
	if err != nil {
		t.Fatalf("ResolveResourceRecord: %v", err)
	}
	if record.Archive != "tar.gz" ||
		len(record.Files) != 1 ||
		record.Files[0].Destination != "components/sample.so" {
		t.Fatalf("record = %+v", record)
	}
}

func TestResolveResourceRecordMapsSingleFile(t *testing.T) {
	content := []byte("plugin")
	resource := manifest.Resource{
		Name: `plugin\.dll$`, Platform: "windows",
	}
	record, err := ResolveResourceRecord(
		context.Background(),
		resourceDownloader{content},
		"plugin://owner/plugin",
		"windows-amd64",
		resource,
		ReleaseAsset{
			Name: "plugin.dll", URL: "https://example.com/plugin.dll",
			Size: int64(len(content)),
		},
	)
	if err != nil {
		t.Fatalf("ResolveResourceRecord: %v", err)
	}
	if record.Archive != "file" ||
		len(record.Files) != 1 ||
		record.Files[0].Destination != "plugins/plugin.dll" {
		t.Fatalf("record = %+v", record)
	}
}

func TestResolveResourceRecordRejectsUnsafeArchive(t *testing.T) {
	archive := zipResource(t, map[string][]byte{"../plugin.so": []byte("bad")})
	resource := manifest.Resource{
		Name: `plugin\.zip$`, Platform: "linux", Archive: true,
		Plugins: []string{"plugin.so"},
	}
	_, err := ResolveResourceRecord(
		context.Background(),
		resourceDownloader{archive},
		"plugin://owner/plugin",
		"linux-amd64",
		resource,
		ReleaseAsset{
			Name: "plugin.zip", URL: "https://example.com/plugin.zip",
			Size: int64(len(archive)),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "unsafe ZIP entry") {
		t.Fatalf("ResolveResourceRecord error = %v", err)
	}
}
