package include

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
)

func TestResolve_QuotedSearchesFromFileFirst(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/filterscripts/session.pwn", []byte(""))
	m.AddFile("/proj/filterscripts/session_utils.inc", []byte("// local override"))
	m.AddFile("/proj/includes/session_utils.inc", []byte("// shared"))

	r := New(m, []string{"/proj/includes"})

	got, ok := r.Resolve("/proj/filterscripts/session.pwn", "session_utils.inc", true)
	if !ok {
		t.Fatal("expected resolution")
	}

	if got != "/proj/filterscripts/session_utils.inc" {
		t.Errorf("got %q, want the file next to the including file to win", got)
	}
}

func TestResolve_QuotedFallsBackToRoots(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/filterscripts/session.pwn", []byte(""))
	m.AddFile("/proj/includes/session_utils.inc", []byte(""))

	r := New(m, []string{"/proj/includes"})

	got, ok := r.Resolve("/proj/filterscripts/session.pwn", "session_utils.inc", true)
	if !ok {
		t.Fatal("expected resolution via include root")
	}

	if got != "/proj/includes/session_utils.inc" {
		t.Errorf("got %q", got)
	}
}

func TestResolve_AngleBracketSkipsFromFileDir(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/gamemodes/main.pwn", []byte(""))
	m.AddFile("/proj/gamemodes/a_samp.inc", []byte("// decoy, must not be used"))
	m.AddFile("/proj/includes/a_samp.inc", []byte("// real"))

	r := New(m, []string{"/proj/includes"})

	got, ok := r.Resolve("/proj/gamemodes/main.pwn", "a_samp", false)
	if !ok {
		t.Fatal("expected resolution")
	}

	if got != "/proj/includes/a_samp.inc" {
		t.Errorf("got %q, angle-bracket include must only search roots", got)
	}
}

func TestResolve_ExtensionAutoAppend(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/includes/a_samp.inc", []byte(""))

	r := New(m, []string{"/proj/includes"})

	got, ok := r.Resolve("/proj/gamemodes/main.pwn", "a_samp", false)
	if !ok || got != "/proj/includes/a_samp.inc" {
		t.Errorf("got (%q, %v)", got, ok)
	}
}

func TestResolve_ExplicitExtensionNotDoubled(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/includes/foo.inc", []byte(""))

	r := New(m, []string{"/proj/includes"})

	got, ok := r.Resolve("/proj/gamemodes/main.pwn", "foo.inc", false)
	if !ok || got != "/proj/includes/foo.inc" {
		t.Errorf("got (%q, %v)", got, ok)
	}
}

func TestResolve_MultipleRootsInOrder(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/vendor/YSI/y_hooks.inc", []byte(""))
	m.AddFile("/proj/includes/y_hooks.inc", []byte("// wrong one"))

	r := New(m, []string{"/proj/vendor/YSI", "/proj/includes"})

	got, ok := r.Resolve("/proj/gamemodes/main.pwn", "y_hooks", false)
	if !ok || got != "/proj/vendor/YSI/y_hooks.inc" {
		t.Errorf("got (%q, %v), want first root to win", got, ok)
	}
}

func TestResolve_NotFound(t *testing.T) {
	m := fsx.NewMem()

	r := New(m, []string{"/proj/includes"})

	if _, ok := r.Resolve("/proj/gamemodes/main.pwn", "nope", false); ok {
		t.Error("expected resolution failure")
	}
}

func TestResolve_TraversalSpecRejected(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/etc/passwd", []byte("root:x:0:0"))

	r := New(m, []string{"/proj/includes"})

	if _, ok := r.Resolve("/proj/gamemodes/main.pwn", "../../../etc/passwd", true); ok {
		t.Error("expected traversal spec to fail resolution, not escape include roots")
	}
}

// TestResolve_PawnCorpusFilterscript uses the optional sibling corpus checkout.
func TestResolve_PawnCorpusFilterscript(t *testing.T) {
	root := filepath.FromSlash("../../pawn-corpus/projects/filterscript-with-includes")

	entry := filepath.Join(root, "filterscripts", "session.pwn")
	if _, err := os.Stat(entry); err != nil {
		t.Skipf("pawn-corpus checkout not available: %v", err)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	fsys := fsx.OS{}
	includeRoot := filepath.ToSlash(filepath.Join(absRoot, "includes"))
	entryAbs := filepath.ToSlash(filepath.Join(absRoot, "filterscripts", "session.pwn"))

	r := New(fsys, []string{includeRoot})

	got, ok := r.Resolve(entryAbs, "session_utils.inc", true)
	if !ok {
		t.Fatal("expected session_utils.inc to resolve via includePaths")
	}

	if filepath.ToSlash(filepath.Clean(includeRoot+"/session_utils.inc")) != got {
		t.Errorf("got %q", got)
	}
}
