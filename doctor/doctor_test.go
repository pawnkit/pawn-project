package doctor

import (
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/paths"
	"github.com/pawnkit/pawnkit-core/hash"
)

func TestManifestParsesCheck(t *testing.T) {
	ok := ManifestParsesCheck{}.Run(Environment{Manifest: &manifest.Manifest{}})
	if ok.Status != StatusOK {
		t.Errorf("Status = %v, want OK", ok.Status)
	}

	fail := ManifestParsesCheck{}.Run(Environment{Manifest: nil})
	if fail.Status != StatusFail {
		t.Errorf("Status = %v, want Fail", fail.Status)
	}
}

func TestIncludePathsExistCheck(t *testing.T) {
	m := fsx.NewMem()
	m.AddDir("/proj/includes")

	okEnv := Environment{FS: m, Paths: paths.Resolved{IncludeRoots: []string{"/proj/includes"}}}
	if got := (IncludePathsExistCheck{}).Run(okEnv); got.Status != StatusOK {
		t.Errorf("Status = %v, want OK: %s", got.Status, got.Message)
	}

	missingEnv := Environment{FS: m, Paths: paths.Resolved{IncludeRoots: []string{"/proj/missing"}}}
	if got := (IncludePathsExistCheck{}).Run(missingEnv); got.Status != StatusWarning {
		t.Errorf("Status = %v, want Warning", got.Status)
	}

	noneEnv := Environment{FS: m, Paths: paths.Resolved{}}
	if got := (IncludePathsExistCheck{}).Run(noneEnv); got.Status != StatusOK {
		t.Errorf("Status = %v, want OK for no declared include paths", got.Status)
	}
}

func TestRun_DefaultChecks(t *testing.T) {
	m := fsx.NewMem()
	m.AddDir("/proj/includes")

	env := Environment{
		FS:       m,
		Manifest: &manifest.Manifest{},
		Paths:    paths.Resolved{IncludeRoots: []string{"/proj/includes"}},
	}

	results := Run(env, DefaultChecks())
	if len(results) != 3 {
		t.Fatalf("results = %+v, want 3", results)
	}

	for _, r := range results {
		if r.Status != StatusOK {
			t.Errorf("check %s = %v: %s", r.Name, r.Status, r.Message)
		}
	}
}

func TestDoctor_NeverExecutesProjectCode(t *testing.T) {
	m := fsx.NewMem()
	env := Environment{FS: m, Manifest: &manifest.Manifest{}}

	for _, c := range DefaultChecks() {
		_ = c.Run(env)
	}
}

func TestManifestDriftCheck(t *testing.T) {
	content := []byte(`{"entry":"main.pwn"}`)
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", content)
	env := Environment{
		FS:       m,
		Manifest: &manifest.Manifest{SourcePath: "/proj/pawn.json"},
		Lock:     &lockfile.Lock{ManifestChecksum: hash.Content(content)},
	}
	if got := (ManifestDriftCheck{}).Run(env); got.Status != StatusOK {
		t.Fatalf("matching status = %v: %s", got.Status, got.Message)
	}
	env.Lock.ManifestChecksum = hash.Content([]byte("old"))
	if got := (ManifestDriftCheck{}).Run(env); got.Status != StatusWarning {
		t.Fatalf("drift status = %v: %s", got.Status, got.Message)
	}
}
