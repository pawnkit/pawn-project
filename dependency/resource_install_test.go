package dependency

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallResourcePayload(t *testing.T) {
	root := t.TempDir()
	writeResourceTestFile(t, root, "plugins/existing.so", "old")

	payload := ResourcePayload{
		"plugins/existing.so":   []byte("new"),
		"includes/./sample.inc": []byte("include"),
	}
	if err := InstallResourcePayload(context.Background(), OSResourceFS{}, root, payload); err != nil {
		t.Fatalf("InstallResourcePayload: %v", err)
	}

	assertResourceTestFile(t, root, "plugins/existing.so", "new")
	assertResourceTestFile(t, root, "includes/sample.inc", "include")
	entries, err := os.ReadDir(filepath.Join(root, ".pawnkit"))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("transaction entries remain: %v", entries)
	}
}

func TestInstallResourcePayloadRollsBack(t *testing.T) {
	root := t.TempDir()
	writeResourceTestFile(t, root, "plugins/a.so", "old-a")
	writeResourceTestFile(t, root, "plugins/b.so", "old-b")

	fsys := &failingResourceFS{failTarget: "plugins/b.so"}
	err := InstallResourcePayload(context.Background(), fsys, root, ResourcePayload{
		"plugins/a.so": []byte("new-a"),
		"plugins/b.so": []byte("new-b"),
	})
	if err == nil {
		t.Fatal("InstallResourcePayload succeeded")
	}
	assertResourceTestFile(t, root, "plugins/a.so", "old-a")
	assertResourceTestFile(t, root, "plugins/b.so", "old-b")
}

func TestInstallResourcePayloadRejectsUnsafeDestinations(t *testing.T) {
	tests := []ResourcePayload{
		{"../outside": []byte("bad")},
		{".pawnkit/state": []byte("bad")},
		{"plugin.inc": nil, "PLUGIN.INC": nil},
	}
	for _, payload := range tests {
		err := InstallResourcePayload(context.Background(), OSResourceFS{}, t.TempDir(), payload)
		if err == nil {
			t.Fatalf("InstallResourcePayload(%v) succeeded", payload)
		}
	}
}

func TestInstallResourcePayloadRejectsSymlinkParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows CI does not grant symlink privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "plugins")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	err := InstallResourcePayload(context.Background(), OSResourceFS{}, root, ResourcePayload{
		"plugins/sample.so": []byte("bad"),
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("InstallResourcePayload error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "sample.so")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside file error = %v", err)
	}
}

func TestInstallResourcePayloadRejectsSymlinkMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows CI does not grant symlink privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".pawnkit")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	err := InstallResourcePayload(context.Background(), OSResourceFS{}, root, ResourcePayload{
		"plugin.so": []byte("bad"),
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("InstallResourcePayload error = %v", err)
	}
}

type failingResourceFS struct {
	OSResourceFS
	failTarget string
	failed     bool
}

func (f *failingResourceFS) Rename(oldPath, newPath string) error {
	if !f.failed && strings.Contains(filepath.ToSlash(oldPath), "/stage/") &&
		strings.HasSuffix(filepath.ToSlash(newPath), f.failTarget) {
		f.failed = true
		return errors.New("injected rename failure")
	}
	return f.OSResourceFS.Rename(oldPath, newPath)
}

func writeResourceTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func assertResourceTestFile(t *testing.T, root, relative, want string) {
	t.Helper()
	content, err := os.ReadFile( //nolint:gosec // Test paths stay below t.TempDir.
		filepath.Join(root, filepath.FromSlash(relative)),
	)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", relative, err)
	}
	if string(content) != want {
		t.Fatalf("ReadFile(%s) = %q, want %q", relative, content, want)
	}
}
