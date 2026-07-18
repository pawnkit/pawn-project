package profile

import (
	"errors"
	"testing"

	"github.com/pawnkit/pawn-project/manifest"
)

func TestSelect_PresetDefaultMapping(t *testing.T) {
	m := &manifest.Manifest{Preset: "openmp"}

	sel, err := Select(m, Options{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel.ProfileID != ProfileOpenMP {
		t.Errorf("ProfileID = %q, want %q", sel.ProfileID, ProfileOpenMP)
	}

	m2 := &manifest.Manifest{Preset: "samp"}

	sel2, err := Select(m2, Options{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel2.ProfileID != ProfileSAMP037 {
		t.Errorf("ProfileID = %q, want %q", sel2.ProfileID, ProfileSAMP037)
	}
}

func TestSelect_PawnKitProfileOverridesPreset(t *testing.T) {
	m := &manifest.Manifest{
		Preset:  "samp",
		PawnKit: &manifest.PawnKitExtension{SchemaVersion: 1, Profile: "strict"},
	}

	sel, err := Select(m, Options{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel.ProfileID != "strict" {
		t.Errorf("ProfileID = %q, want strict", sel.ProfileID)
	}
}

func TestSelect_CLIOverrideWinsOverEverything(t *testing.T) {
	m := &manifest.Manifest{
		Preset:  "samp",
		PawnKit: &manifest.PawnKitExtension{SchemaVersion: 1, Profile: "strict"},
	}

	sel, err := Select(m, Options{ProfileOverride: "legacy"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel.ProfileID != "legacy" {
		t.Errorf("ProfileID = %q, want legacy", sel.ProfileID)
	}
}

func TestSelect_NoProfileDeterminable(t *testing.T) {
	sel, err := Select(&manifest.Manifest{}, Options{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel.ProfileID != "" {
		t.Errorf("ProfileID = %q, want empty", sel.ProfileID)
	}
}

func TestSelect_SingleBuildAndRuntime(t *testing.T) {
	m := &manifest.Manifest{
		Build:   &manifest.Build{Args: []string{"-d3"}},
		Runtime: &manifest.Runtime{Port: 7777},
	}

	sel, err := Select(m, Options{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel.Build == nil || sel.Build.Args[0] != "-d3" {
		t.Errorf("Build = %+v", sel.Build)
	}

	if sel.Runtime == nil || sel.Runtime.Port != 7777 {
		t.Errorf("Runtime = %+v", sel.Runtime)
	}
}

func TestSelect_NamedBuild(t *testing.T) {
	m := &manifest.Manifest{
		Builds: []manifest.Build{
			{Name: "debug", Args: []string{"-d3"}},
			{Name: "release", Args: []string{"-O2"}},
		},
	}

	sel, err := Select(m, Options{BuildName: "release"})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel.Build == nil || sel.Build.Name != "release" {
		t.Errorf("Build = %+v", sel.Build)
	}

	// No explicit name: first entry wins.
	sel2, err := Select(m, Options{})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if sel2.Build.Name != "debug" {
		t.Errorf("default Build = %+v, want first entry", sel2.Build)
	}
}

func TestSelect_UnknownBuildName(t *testing.T) {
	m := &manifest.Manifest{Builds: []manifest.Build{{Name: "debug"}}}

	_, err := Select(m, Options{BuildName: "nope"})
	if !errors.Is(err, ErrBuildNotFound) {
		t.Fatalf("err = %v, want ErrBuildNotFound", err)
	}
}

func TestSelect_UnknownRuntimeName(t *testing.T) {
	m := &manifest.Manifest{Runtimes: []manifest.Runtime{{Name: "prod"}}}

	_, err := Select(m, Options{RuntimeName: "nope"})
	if !errors.Is(err, ErrRuntimeNotFound) {
		t.Fatalf("err = %v, want ErrRuntimeNotFound", err)
	}
}
