package project

import (
	"sync"
	"testing"

	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
)

func buildFixture() *fsx.Mem {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{
		"entry": "gamemodes/main.pwn",
		"output": "gamemodes/main.amx",
		"preset": "openmp",
		"include_path": "includes",
		"pawnkit": {"schemaVersion": 1}
	}`))
	m.AddFile("/proj/gamemodes/main.pwn", []byte(`#include <a_samp>
#include "helper.inc"
main() {}`))
	m.AddFile("/proj/includes/a_samp.inc", []byte("// stub"))
	m.AddFile("/proj/includes/helper.inc", []byte("stock helper() {}"))
	m.AddFile("/proj/pawn.lock", []byte(`{"schemaVersion": 1, "packages": []}`))

	return m
}

func TestLoad_FullyResolvedProject(t *testing.T) {
	m := buildFixture()

	p, err := Load(source.NewRegistry(), m, "/proj/gamemodes/main.pwn", Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if p.Root() != "/proj" {
		t.Errorf("Root = %q", p.Root())
	}

	if p.Manifest() == nil || p.Manifest().Entry != "gamemodes/main.pwn" {
		t.Errorf("Manifest = %+v", p.Manifest())
	}

	if p.Selection().ProfileID != "openmp" {
		t.Errorf("ProfileID = %q", p.Selection().ProfileID)
	}

	if p.Paths().Entry != "/proj/gamemodes/main.pwn" {
		t.Errorf("Paths.Entry = %q", p.Paths().Entry)
	}

	if p.Lockfile() == nil {
		t.Fatal("expected lockfile to be loaded")
	}

	if len(p.Diagnostics()) != 0 {
		t.Errorf("Diagnostics = %+v", p.Diagnostics())
	}

	resolved, ok := p.IncludeResolver().Resolve("/proj/gamemodes/main.pwn", "helper.inc", true)
	if !ok || resolved != "/proj/includes/helper.inc" {
		t.Errorf("Resolve(helper.inc) = (%q, %v)", resolved, ok)
	}

	fp, err := p.Fingerprint()
	if err != nil {
		t.Fatalf("Fingerprint: %v", err)
	}

	if fp == "" {
		t.Error("Fingerprint is empty")
	}
}

func TestLoad_NoLockfileIsFine(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"entry": "a.pwn"}`))
	m.AddFile("/proj/a.pwn", []byte(""))

	p, err := Load(source.NewRegistry(), m, "/proj/a.pwn", Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if p.Lockfile() != nil {
		t.Errorf("Lockfile = %+v, want nil", p.Lockfile())
	}
}

func TestLoad_NoManifestFound(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/a.pwn", []byte(""))

	if _, err := Load(source.NewRegistry(), m, "/proj/a.pwn", Options{}); err == nil {
		t.Fatal("expected error when no manifest is found")
	}
}

func TestLoad_MalformedManifestStillProducesUsableProject(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{not valid json`))
	m.AddFile("/proj/a.pwn", []byte(""))

	p, err := Load(source.NewRegistry(), m, "/proj/a.pwn", Options{})
	if err != nil {
		t.Fatalf("Load should degrade gracefully, got error: %v", err)
	}

	if p.Manifest() != nil {
		t.Error("Manifest should be nil for unparsable content")
	}

	if len(p.Diagnostics()) == 0 {
		t.Error("expected a diagnostic explaining the parse failure")
	}

	if p.Root() != "/proj" {
		t.Errorf("Root = %q, want /proj even with a malformed manifest", p.Root())
	}
}

func TestLoad_ProfileOverride(t *testing.T) {
	m := buildFixture()

	p, err := Load(source.NewRegistry(), m, "/proj", Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if p.Selection().ProfileID != "openmp" {
		t.Fatalf("ProfileID = %q", p.Selection().ProfileID)
	}
}

// TestProject_ConcurrentReads checks immutable project access under -race.
func TestProject_ConcurrentReads(t *testing.T) {
	m := buildFixture()

	p, err := Load(source.NewRegistry(), m, "/proj/gamemodes/main.pwn", Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	const goroutines = 32

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			_ = p.Root()
			_ = p.Manifest()
			_ = p.Lockfile()
			_ = p.Selection()
			_ = p.Paths()
			_ = p.Diagnostics()

			if _, err := p.Fingerprint(); err != nil {
				t.Errorf("Fingerprint: %v", err)
			}

			if _, ok := p.IncludeResolver().Resolve("/proj/gamemodes/main.pwn", "helper.inc", true); !ok {
				t.Error("Resolve(helper.inc) failed")
			}

			if _, ok := p.IncludeResolver().Resolve("/proj/gamemodes/main.pwn", "a_samp", false); !ok {
				t.Error("Resolve(a_samp) failed")
			}
		}()
	}

	wg.Wait()
}
