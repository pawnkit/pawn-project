package dependency

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
)

func TestResourceResolverBuildsTargetRecordsAndPreservesOthers(t *testing.T) {
	archive := zipResource(t, map[string][]byte{
		"pawno/include/plugin.inc": []byte("include"),
		"plugins/plugin.so":        []byte("plugin"),
	})
	mem := fsx.NewMem()
	mem.AddFile("/project/dependencies/plugin/pawn.json", []byte(`{
		"resources": [{
			"name": "^plugin-linux.zip$",
			"platform": "linux",
			"archive": true,
			"includes": ["pawno/include"],
			"plugins": ["plugins/plugin.so"]
		}]
	}`))
	pkg := lockfile.Package{
		Key: "github.com/owner/plugin", Name: "owner/plugin",
		Resolved: "v1", Commit: "abcdef0", Kind: lockfile.KindDependency,
		Source: lockfile.PackageSource{
			Type: lockfile.SourceTypeGit,
			URL:  "https://github.com/owner/plugin",
		},
	}
	windows := testResource("file", []byte("windows"), lockfile.ResolvedResourceFile{
		Source: "plugin.dll", Destination: "plugins/plugin.dll",
		Size: 7, Checksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	windows.Package = "github.com/owner/plugin"
	windows.Target = "windows-amd64"
	lock := &lockfile.Lock{
		SchemaVersion: 1,
		Packages:      []lockfile.Package{pkg},
		Resources:     []lockfile.ResolvedResource{windows},
	}
	provider := &recordingReleaseProvider{assets: []ReleaseAsset{{
		Name: "plugin-linux.zip", URL: "https://example.com/plugin.zip",
		Size: int64(len(archive)),
	}}}
	downloader := keyedResourceDownloader{
		"https://example.com/plugin.zip": archive,
	}

	records, err := NewResourceResolver(mem, downloader, provider).
		Resolve(context.Background(), "/project", "linux-amd64", "", lock)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if records[0].Target != "linux-amd64" || records[1].Target != "windows-amd64" {
		t.Fatalf("targets = %s, %s", records[0].Target, records[1].Target)
	}
	if records[0].Package != "github.com/owner/plugin" {
		t.Fatalf("package key = %q", records[0].Package)
	}
	if len(provider.packages) != 1 || provider.packages[0].Resolved != "v1" {
		t.Fatalf("provider packages = %+v", provider.packages)
	}
}

func TestResourceResolverReportsMissingManifestResources(t *testing.T) {
	mem := fsx.NewMem()
	mem.AddFile("/project/dependencies/plugin/pawn.json", []byte(`{"resources":[]}`))
	lock := &lockfile.Lock{Packages: []lockfile.Package{{
		Name: "owner/plugin", Kind: lockfile.KindPlugin,
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit},
	}}}

	_, err := NewResourceResolver(mem, keyedResourceDownloader{}, &recordingReleaseProvider{}).
		Resolve(context.Background(), "/project", "linux-amd64", "", lock)
	if err == nil {
		t.Fatal("Resolve succeeded")
	}
}

func TestResourceResolverRecognizesCompleteTarget(t *testing.T) {
	mem := fsx.NewMem()
	mem.AddFile("/project/dependencies/plugin/pawn.json", []byte(`{
		"resources": [{
			"name": "plugin.zip",
			"platform": "linux",
			"archive": true
		}]
	}`))
	pkg := lockfile.Package{
		Key: "github.com/owner/plugin", Name: "owner/plugin",
		Kind:   lockfile.KindDependency,
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit},
	}
	lock := &lockfile.Lock{
		Packages: []lockfile.Package{pkg},
		Resources: []lockfile.ResolvedResource{{
			Package: "github.com/owner/plugin", Resource: "plugin.zip",
			Target: "linux-amd64",
		}},
	}
	resolver := NewResourceResolver(mem, nil, nil)

	complete, err := resolver.HasCompleteTarget(
		context.Background(), "/project", "linux-amd64", "", lock,
	)
	if err != nil || !complete {
		t.Fatalf("HasCompleteTarget = %v, %v", complete, err)
	}
	lock.Resources[0].Target = "windows-amd64"
	complete, err = resolver.HasCompleteTarget(
		context.Background(), "/project", "linux-amd64", "", lock,
	)
	if err != nil || complete {
		t.Fatalf("HasCompleteTarget missing = %v, %v", complete, err)
	}
}

type recordingReleaseProvider struct {
	assets   []ReleaseAsset
	packages []lockfile.Package
}

func (p *recordingReleaseProvider) Assets(
	_ context.Context,
	pkg lockfile.Package,
) ([]ReleaseAsset, error) {
	p.packages = append(p.packages, pkg)
	return p.assets, nil
}

type keyedResourceDownloader map[string][]byte

func (d keyedResourceDownloader) Download(
	_ context.Context,
	url string,
) (io.ReadCloser, error) {
	content, ok := d[url]
	if !ok {
		return nil, fmt.Errorf("unexpected URL %s", url)
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}
