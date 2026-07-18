package paths

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/manifest"
)

func TestResolve_Basic(t *testing.T) {
	m := &manifest.Manifest{
		Entry:       "gamemodes/main.pwn",
		Output:      "gamemodes/main.amx",
		IncludePath: "include",
		PawnKit:     &manifest.PawnKitExtension{SchemaVersion: 1, IncludePaths: []string{"vendor/include"}},
	}

	r, err := Resolve("/proj", m)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.Entry != "/proj/gamemodes/main.pwn" {
		t.Errorf("Entry = %q", r.Entry)
	}

	if r.Output != "/proj/gamemodes/main.amx" {
		t.Errorf("Output = %q", r.Output)
	}

	want := []string{"/proj/include", "/proj/vendor/include"}
	if len(r.IncludeRoots) != len(want) {
		t.Fatalf("IncludeRoots = %v", r.IncludeRoots)
	}

	for i := range want {
		if r.IncludeRoots[i] != want[i] {
			t.Errorf("IncludeRoots[%d] = %q, want %q", i, r.IncludeRoots[i], want[i])
		}
	}

	if len(r.GeneratedFiles) != 1 || !strings.HasSuffix(r.GeneratedFiles[0], "sampctl_build_file.inc") {
		t.Errorf("GeneratedFiles = %v", r.GeneratedFiles)
	}
}

func TestResolve_BuildFileDisabled(t *testing.T) {
	no := false
	m := &manifest.Manifest{Experimental: manifest.Experimental{BuildFile: &no}}

	r, err := Resolve("/proj", m)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.GeneratedFiles) != 0 {
		t.Errorf("GeneratedFiles = %v, want none", r.GeneratedFiles)
	}
}

func TestResolve_TraversalRejected(t *testing.T) {
	m := &manifest.Manifest{Entry: "../../etc/passwd"}

	if _, err := Resolve("/proj", m); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestResolve_WindowsStyleRelativePaths(t *testing.T) {
	m := &manifest.Manifest{Entry: `gamemodes\main.pwn`, IncludePath: `vendor\include`}

	r, err := Resolve("/proj", m)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.Entry != "/proj/gamemodes/main.pwn" {
		t.Errorf("Entry = %q", r.Entry)
	}

	if len(r.IncludeRoots) != 1 || r.IncludeRoots[0] != "/proj/vendor/include" {
		t.Errorf("IncludeRoots = %v", r.IncludeRoots)
	}
}

func TestResolve_WindowsRoot(t *testing.T) {
	m := &manifest.Manifest{Entry: "main.pwn"}

	r, err := Resolve(`C:\projects\proj`, m)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if r.Entry != "C:/projects/proj/main.pwn" {
		t.Errorf("Entry = %q", r.Entry)
	}
}

func TestResolve_DedupesIncludeRoots(t *testing.T) {
	m := &manifest.Manifest{
		IncludePath: "include",
		PawnKit:     &manifest.PawnKitExtension{SchemaVersion: 1, IncludePaths: []string{"include"}},
	}

	r, err := Resolve("/proj", m)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(r.IncludeRoots) != 1 {
		t.Errorf("IncludeRoots = %v, want deduped to 1", r.IncludeRoots)
	}
}
