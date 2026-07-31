package dependency

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/manifest"
)

func TestSelectManifestResourceUsesVersionThenPlatformDefault(t *testing.T) {
	resources := []manifest.Resource{
		{Name: "windows", Platform: "windows"},
		{Name: "linux-default", Platform: "linux"},
		{Name: "linux-openmp", Platform: "linux", Version: "openmp"},
	}

	selected, err := SelectManifestResource(resources, "linux-amd64", "openmp")
	if err != nil {
		t.Fatalf("SelectManifestResource: %v", err)
	}
	if selected.Name != "linux-openmp" {
		t.Fatalf("selected = %q, want linux-openmp", selected.Name)
	}

	selected, err = SelectManifestResource(resources, "linux-amd64", "unknown")
	if err != nil {
		t.Fatalf("SelectManifestResource fallback: %v", err)
	}
	if selected.Name != "linux-default" {
		t.Fatalf("fallback = %q, want linux-default", selected.Name)
	}
}

func TestSelectManifestResourceRequiresExactPlatform(t *testing.T) {
	_, err := SelectManifestResource(
		[]manifest.Resource{{Name: "linux", Platform: "linux"}},
		"windows-amd64",
		"",
	)
	if err == nil {
		t.Fatal("SelectManifestResource succeeded")
	}
}

func TestSelectReleaseAssetUsesRegularExpression(t *testing.T) {
	asset, err := SelectReleaseAsset(`^plugin-(.*)\.zip$`, []ReleaseAsset{
		{Name: "source.zip", URL: "https://example.com/source.zip", Size: 10},
		{Name: "plugin-v1.zip", URL: "https://example.com/plugin.zip", Size: 20},
	})
	if err != nil {
		t.Fatalf("SelectReleaseAsset: %v", err)
	}
	if asset.Name != "plugin-v1.zip" {
		t.Fatalf("asset = %q, want plugin-v1.zip", asset.Name)
	}
}

func TestSelectReleaseAssetUsesProviderOrderAndRejectsUnsafeURLs(t *testing.T) {
	asset, err := SelectReleaseAsset(`^plugin-`, []ReleaseAsset{
		{Name: "plugin-a", URL: "https://example.com/a", Size: 1},
		{Name: "plugin-b", URL: "https://example.com/b", Size: 1},
	})
	if err != nil {
		t.Fatalf("SelectReleaseAsset: %v", err)
	}
	if asset.Name != "plugin-a" {
		t.Fatalf("asset = %q, want plugin-a", asset.Name)
	}

	_, err = SelectReleaseAsset(`plugin`, []ReleaseAsset{
		{Name: "plugin", URL: "http://example.com/plugin", Size: 1},
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe URL") {
		t.Fatalf("URL error = %v", err)
	}
}
