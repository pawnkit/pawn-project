package dependency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
)

func TestValidateGitSource(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "HTTPS", url: "https://github.com/pawnkit/pawn-project", ok: true},
		{name: "HTTP", url: "http://github.com/pawnkit/pawn-project"},
		{name: "SSH", url: "git@github.com:pawnkit/pawn-project.git"},
		{name: "credentials", url: "https://token@github.com/pawnkit/pawn-project"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateGitSource(test.url)
			if (err == nil) != test.ok {
				t.Fatalf("validateGitSource(%q) error = %v", test.url, err)
			}
		})
	}
}

func TestValidGitCommit(t *testing.T) {
	for commit, want := range map[string]bool{
		"abcdef0":               true,
		strings.Repeat("a", 40): true,
		"ABCDEF0":               false,
		"../bad":                false,
		"short":                 false,
	} {
		if got := validGitCommit(commit); got != want {
			t.Errorf("validGitCommit(%q) = %t, want %t", commit, got, want)
		}
	}
}

func TestLimitedBuffer(t *testing.T) {
	input := make([]byte, maxGitOutput+20)
	buffer := &limitedBuffer{}
	written, err := buffer.Write(input)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if written != len(input) {
		t.Fatalf("written = %d, want %d", written, len(input))
	}
	if got := len(buffer.String()); got != maxGitOutput {
		t.Fatalf("buffer length = %d, want %d", got, maxGitOutput)
	}
}

type fakeGitRunner struct {
	commit      string
	cloneSource []string
}

func (r *fakeGitRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "clone" {
		r.cloneSource = append(r.cloneSource, args[len(args)-2])
		if err := os.MkdirAll(args[len(args)-1], 0o750); err != nil {
			return "", err
		}
		return "", nil
	}
	if len(args) > 0 && args[len(args)-1] == "HEAD" {
		return r.commit + "\n", nil
	}
	if strings.HasSuffix(args[len(args)-1], "^{commit}") {
		return r.commit + "\n", nil
	}
	return "", nil
}

func TestGitInstallerStagesExactCommit(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "dependencies", "example")
	commit := strings.Repeat("a", 40)
	runner := &fakeGitRunner{commit: commit}
	installer := GitInstaller{runner: runner}
	pkg := lockfile.Package{
		Name:   "owner/example",
		Commit: commit,
		Source: lockfile.PackageSource{
			Type: lockfile.SourceTypeGit,
			URL:  "https://github.com/owner/example",
		},
	}

	status, err := installer.Install(context.Background(), pkg, target)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if status != StatusInstalled {
		t.Fatalf("status = %q, want %q", status, StatusInstalled)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("target was not installed: %v", err)
	}

	status, err = installer.Install(context.Background(), pkg, target)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if status != StatusPresent {
		t.Fatalf("second status = %q, want %q", status, StatusPresent)
	}
}

func TestGitInstallerReusesPersistentCache(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	commit := strings.Repeat("a", 40)
	runner := &fakeGitRunner{commit: commit}
	installer := GitInstaller{CacheDir: cache, runner: runner}
	pkg := lockfile.Package{
		Name: "owner/example", Commit: commit,
		Source: lockfile.PackageSource{
			Type: lockfile.SourceTypeGit, URL: "https://github.com/owner/example",
		},
	}

	if _, err := installer.Install(context.Background(), pkg, filepath.Join(root, "first")); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(context.Background(), pkg, filepath.Join(root, "second")); err != nil {
		t.Fatal(err)
	}
	if len(runner.cloneSource) != 3 {
		t.Fatalf("clone sources = %v", runner.cloneSource)
	}
	if runner.cloneSource[0] != pkg.Source.URL || runner.cloneSource[1] == pkg.Source.URL ||
		runner.cloneSource[2] != runner.cloneSource[1] {
		t.Fatalf("clone sources = %v", runner.cloneSource)
	}
}

func TestGitInstallerRejectsSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Symlink(root, target); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	pkg := lockfile.Package{
		Name: "owner/example", Commit: strings.Repeat("a", 40),
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/repo"},
	}
	if _, err := (GitInstaller{}).Install(context.Background(), pkg, target); err == nil {
		t.Fatal("symlink target was accepted")
	}
}

func TestVerifyDirectoryIntegrity(t *testing.T) {
	root := t.TempDir()
	content := []byte("main() {}\n")
	if err := os.WriteFile(filepath.Join(root, "main.pwn"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("main.pwn"))
	_, _ = hash.Write(content)
	expected := "sha256:" + hex.EncodeToString(hash.Sum(nil))

	if err := verifyDirectoryIntegrity(root, expected); err != nil {
		t.Fatalf("verifyDirectoryIntegrity: %v", err)
	}
	if err := verifyDirectoryIntegrity(root, "sha256:"+strings.Repeat("0", 64)); err == nil {
		t.Fatal("mismatched integrity passed")
	}
}
