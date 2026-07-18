package workspace

import (
	"errors"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
)

func TestFindRoot_FromNestedFile(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{}`))
	m.AddFile("/proj/gamemodes/main.pwn", []byte(""))

	root, err := FindRoot(m, "/proj/gamemodes/main.pwn")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}

	if root.Dir != "/proj" {
		t.Errorf("Dir = %q, want /proj", root.Dir)
	}

	if root.ManifestPath != "/proj/pawn.json" {
		t.Errorf("ManifestPath = %q", root.ManifestPath)
	}

	if root.ManifestName != ManifestJSON {
		t.Errorf("ManifestName = %q", root.ManifestName)
	}
}

func TestFindRoot_FromDirectory(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.yaml", []byte("entry: a.pwn\n"))
	m.AddDir("/proj/deep/nested")

	root, err := FindRoot(m, "/proj/deep/nested")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}

	if root.Dir != "/proj" || root.ManifestName != ManifestYAML {
		t.Errorf("root = %+v", root)
	}
}

func TestFindRoot_ManifestAtParent(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/workspace/pawn.json", []byte(`{}`))
	m.AddFile("/workspace/a/b/c/file.pwn", []byte(""))

	root, err := FindRoot(m, "/workspace/a/b/c/file.pwn")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}

	if root.Dir != "/workspace" {
		t.Errorf("Dir = %q, want /workspace", root.Dir)
	}
}

func TestFindRoot_JSONWinsOverYAML(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{}`))
	m.AddFile("/proj/pawn.yaml", []byte(""))

	root, err := FindRoot(m, "/proj")
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}

	if root.ManifestName != ManifestJSON {
		t.Errorf("ManifestName = %q, want pawn.json to win", root.ManifestName)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/somewhere/file.pwn", []byte(""))

	_, err := FindRoot(m, "/somewhere/file.pwn")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestFindRoot_RequiresAbsolute(t *testing.T) {
	m := fsx.NewMem()

	if _, err := FindRoot(m, "relative/path"); err == nil {
		t.Fatal("expected error for relative start path")
	}
}
