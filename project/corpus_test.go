package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
)

func TestLoadCorpusProjects(t *testing.T) {
	root := projectCorpusRoot()
	if root == "" {
		t.Skip("pawn-corpus is unavailable")
	}
	tests := []struct {
		name    string
		profile string
		include string
	}{
		{name: "minimal-gamemode", profile: "openmp"},
		{name: "minimal-samp-gamemode", profile: "samp-037", include: "a_samp"},
		{name: "filterscript-with-includes", profile: "openmp", include: "session_utils"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectRoot := filepath.Join(root, "projects", test.name)
			p, err := Load(source.NewRegistry(), fsx.OS{}, projectRoot, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if p.Selection().ProfileID != test.profile {
				t.Fatalf("profile = %q, want %q", p.Selection().ProfileID, test.profile)
			}
			if p.Paths().Entry == "" {
				t.Fatal("entry was not resolved")
			}
			if _, err := os.Stat(p.Paths().Entry); err != nil {
				t.Fatalf("entry: %v", err)
			}
			if len(p.Diagnostics()) != 0 {
				t.Fatalf("diagnostics = %+v", p.Diagnostics())
			}
			if test.include != "" {
				if _, ok := p.IncludeResolver().Resolve(p.Paths().Entry, test.include, false); !ok {
					t.Fatalf("include %q was not resolved", test.include)
				}
			}
		})
	}
}

func projectCorpusRoot() string {
	if root := os.Getenv("PAWN_CORPUS_DIR"); root != "" {
		//nolint:gosec // Test fixture path.
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			return root
		}
		return ""
	}
	root := filepath.Join("..", "..", "pawn-corpus")
	absolute, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	if info, err := os.Stat(absolute); err == nil && info.IsDir() {
		return absolute
	}
	return ""
}
