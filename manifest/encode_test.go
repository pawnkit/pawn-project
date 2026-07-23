package manifest

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
)

func TestNewAndEncodeJSON(t *testing.T) {
	t.Parallel()

	project, err := New(CreateOptions{
		Entry:        "gamemodes/main.pwn",
		Profile:      ProfileOpenMP,
		IncludePaths: []string{"include"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	content, err := EncodeJSON(project)
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}

	want := `{
  "entry": "gamemodes/main.pwn",
  "preset": "openmp",
  "experimental": {
    "build_file": false
  },
  "pawnkit": {
    "schemaVersion": 1,
    "profile": "openmp",
    "includePaths": [
      "include"
    ]
  }
}
`
	if string(content) != want {
		t.Fatalf("EncodeJSON:\n%s\nwant:\n%s", content, want)
	}

	memory := fsx.NewMem()
	memory.AddFile("/project/pawn.json", content)
	result, err := Load(source.NewRegistry(), memory, "/project/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", result.Diagnostics)
	}
}

func TestEncodeJSONPreservesDependencies(t *testing.T) {
	t.Parallel()

	dependency, err := ParseDependency("pawn-lang/YSI-Includes:5.10.0")
	if err != nil {
		t.Fatal(err)
	}
	content, err := EncodeJSON(&Manifest{Dependencies: []Dependency{dependency}})
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	if !strings.Contains(string(content), `"pawn-lang/YSI-Includes:5.10.0"`) {
		t.Fatalf("dependency missing from %s", content)
	}
}

func TestNewRejectsIncompleteManifest(t *testing.T) {
	t.Parallel()

	if _, err := New(CreateOptions{Profile: ProfileOpenMP}); err == nil {
		t.Fatal("expected a missing-entry error")
	}
	if _, err := New(CreateOptions{Entry: "main.pwn", Profile: "Not Valid"}); err == nil {
		t.Fatal("expected an invalid-profile error")
	}
}
