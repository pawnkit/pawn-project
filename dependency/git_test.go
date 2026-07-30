package dependency

import (
	"context"
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
	commit string
}

func (r fakeGitRunner) Run(_ context.Context, _ string, args ...string) (string, error) {
	if len(args) > 0 && args[0] == "clone" {
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
	installer := GitInstaller{runner: fakeGitRunner{commit: commit}}
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
