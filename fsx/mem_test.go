package fsx

import (
	"errors"
	"io/fs"
	"testing"
)

func TestMemBasics(t *testing.T) {
	m := NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{}`))
	m.AddFile("/proj/includes/foo.inc", []byte("stock x() {}"))
	m.AddDir("/proj/empty")

	if !IsFile(m, "/proj/pawn.json") {
		t.Error("expected pawn.json to be a file")
	}

	if !IsDir(m, "/proj") {
		t.Error("expected /proj to be a dir")
	}

	if !IsDir(m, "/proj/empty") {
		t.Error("expected /proj/empty to be a dir")
	}

	content, err := m.ReadFile("/proj/pawn.json")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(content) != "{}" {
		t.Errorf("content = %q", content)
	}

	entries, err := m.ReadDir("/proj")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}

	want := []string{"empty", "includes", "pawn.json"}
	if len(names) != len(want) {
		t.Fatalf("ReadDir names = %v, want %v", names, want)
	}

	for i := range want {
		if names[i] != want[i] {
			t.Errorf("ReadDir names = %v, want %v", names, want)

			break
		}
	}
}

func TestMemNotExist(t *testing.T) {
	m := NewMem()

	_, err := m.Stat("/nope")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat error = %v, want ErrNotExist", err)
	}

	if Exists(m, "/nope") {
		t.Error("Exists should be false")
	}
}
