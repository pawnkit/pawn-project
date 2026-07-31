package dependency

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/manifest"
)

type revisionGitRunner struct {
	commit string
	tags   string
}

func (r revisionGitRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "clone" {
		checkout := args[len(args)-1]
		if err := os.MkdirAll(checkout, 0o750); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(checkout, "pawn.json"), []byte(`{"dependencies":[]}`), 0o600); err != nil {
			return "", err
		}
		return "", nil
	}
	if len(args) >= 2 && args[len(args)-2] == "tag" {
		return r.tags, nil
	}
	if len(args) >= 2 && args[len(args)-2] == "rev-parse" {
		return r.commit + "\n", nil
	}
	return "", nil
}

func TestGitRevisionProviderResolvesRange(t *testing.T) {
	dep, err := manifest.ParseDependency("https://gitlab.com/example/library:^1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	provider := GitRevisionProvider{runner: revisionGitRunner{
		commit: strings.Repeat("a", 40),
		tags:   "v1.0.0\nv1.4.0\nv2.0.0\n",
	}}
	revision, err := provider.Resolve(context.Background(), dep, nil)
	if err != nil {
		t.Fatal(err)
	}
	if revision.Resolved != "v1.4.0" || revision.CanonicalSite != "gitlab.com" ||
		revision.SourceURL != "https://gitlab.com/example/library" {
		t.Fatalf("revision = %+v", revision)
	}
}

func TestLoadGitRevisionManifestRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "pawn.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := loadGitRevisionManifest(root); err == nil {
		t.Fatal("symlink manifest was accepted")
	}
}

func TestLoadGitRevisionManifestRejectsOversizedFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "pawn.json")
	file, err := os.Create(path) //nolint:gosec // Test path is inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxDependencyManifestBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadGitRevisionManifest(root); err == nil {
		t.Fatal("oversized manifest was accepted")
	}
}

func TestValidGitCheckoutRef(t *testing.T) {
	for ref, want := range map[string]bool{
		gitHead:                 true,
		"refs/tags/v1.2.3":      true,
		strings.Repeat("a", 40): true,
		"--help":                false,
		"bad..ref":              false,
		"bad ref":               false,
	} {
		if got := validGitCheckoutRef(ref); got != want {
			t.Errorf("validGitCheckoutRef(%q) = %t, want %t", ref, got, want)
		}
	}
}
