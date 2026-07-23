package project

import (
	"slices"
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

	if !slices.Contains(p.Paths().IncludeRoots, "/proj/gamemodes") {
		t.Errorf("Paths.IncludeRoots = %q", p.Paths().IncludeRoots)
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

func TestLoad_QuotedIncludesUseEntryDirectory(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"entry":"gamemodes/gamemode.pwn"}`))
	m.AddFile("/proj/gamemodes/gamemode.pwn", []byte(""))
	m.AddFile("/proj/gamemodes/modules/player/main.pwn", []byte(""))
	m.AddFile("/proj/gamemodes/modules/player/joining.pwn", []byte(""))

	p, err := Load(source.NewRegistry(), m, "/proj/gamemodes/modules/player/main.pwn", Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, ok := p.IncludeResolver().Resolve(
		"/proj/gamemodes/modules/player/main.pwn",
		"modules/player/joining.pwn",
		true,
	)
	if !ok || got != "/proj/gamemodes/modules/player/joining.pwn" {
		t.Fatalf("Resolve() = (%q, %v)", got, ok)
	}
}

func TestLoad_UsesSampctlIncludeLayout(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{
		"entry":"gamemodes/main.pwn",
		"build":{"includes":["legacy"]},
		"dependencies":["sampctl/samp-stdlib","example/pawn-map"]
	}`))
	m.AddFile("/proj/gamemodes/main.pwn", []byte(""))
	m.AddFile("/proj/legacy/legacy.inc", []byte(""))
	m.AddFile("/proj/local.inc", []byte(""))
	m.AddFile("/proj/dependencies/samp-stdlib/a_samp.inc", []byte(""))
	m.AddFile("/proj/dependencies/pawn-map/pawn.json", []byte(`{"include_path":"include"}`))
	m.AddFile("/proj/dependencies/pawn-map/include/map.inc", []byte(""))
	m.AddFile("/proj/dependencies/.resources/sscanf/sscanf2.inc", []byte(""))

	p, err := Load(source.NewRegistry(), m, "/proj", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"/proj/legacy",
		"/proj/gamemodes",
		"/proj",
		"/proj/dependencies/.resources/sscanf",
		"/proj/dependencies/pawn-map/include",
		"/proj/dependencies/samp-stdlib",
	}
	if got := p.Paths().IncludeRoots; !slices.Equal(got, want) {
		t.Fatalf("include roots = %v, want %v", got, want)
	}
	for _, include := range []string{"legacy", "local", "map", "a_samp", "sscanf2"} {
		if _, ok := p.IncludeResolver().Resolve(p.Paths().Entry, include, false); !ok {
			t.Errorf("include %q was not resolved", include)
		}
	}
}

func TestLoad_DependencyWithLegacyResourceShape(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"entry":"gamemodes/main.pwn"}`))
	m.AddFile("/proj/gamemodes/main.pwn", []byte(""))
	m.AddFile("/proj/dependencies/memory/pawn.json", []byte(`{"resources":[]}`))
	m.AddFile("/proj/dependencies/memory/include/memory.inc", []byte(""))

	p, err := Load(source.NewRegistry(), m, "/proj", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.IncludeResolver().Resolve(p.Paths().Entry, "memory", false); !ok {
		t.Fatal("dependency include fallback was not resolved")
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
