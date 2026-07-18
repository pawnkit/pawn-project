package toolconfig

import (
	"reflect"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/manifest"
)

func TestStandaloneFileName(t *testing.T) {
	if got := StandaloneFileName("pawnlint"); got != "pawnlint.json" {
		t.Errorf("pawnlint = %q", got)
	}

	if got := StandaloneFileName("pawnfmt"); got != ".pawnfmt.json" {
		t.Errorf("pawnfmt = %q", got)
	}
}

func TestMerge_ScalarStandaloneWins(t *testing.T) {
	standalone := map[string]any{"profile": "strict"}
	embedded := map[string]any{"profile": "recommended", "extends": "base"}

	got := Merge(standalone, embedded)

	want := map[string]any{"profile": "strict", "extends": "base"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Merge = %+v, want %+v", got, want)
	}
}

func TestMerge_ArraysConcatDeduped(t *testing.T) {
	standalone := map[string]any{"ignore": []any{"a.pwn", "b.pwn"}}
	embedded := map[string]any{"ignore": []any{"b.pwn", "c.pwn"}}

	got := Merge(standalone, embedded)

	want := []any{"a.pwn", "b.pwn", "c.pwn"}
	if !reflect.DeepEqual(got["ignore"], want) {
		t.Errorf("ignore = %+v, want %+v", got["ignore"], want)
	}
}

func TestMerge_NestedObjectsRecurse(t *testing.T) {
	standalone := map[string]any{"rules": map[string]any{"no-goto": "error"}}
	embedded := map[string]any{"rules": map[string]any{"no-goto": "warning", "no-eval": "error"}}

	got := Merge(standalone, embedded)

	rules, ok := got["rules"].(map[string]any)
	if !ok {
		t.Fatalf("rules = %v", got["rules"])
	}

	if rules["no-goto"] != "error" {
		t.Errorf("no-goto = %v, want standalone's error to win", rules["no-goto"])
	}

	if rules["no-eval"] != "error" {
		t.Errorf("no-eval = %v, want embedded-only key preserved", rules["no-eval"])
	}
}

func TestMerge_BothNilReturnsNil(t *testing.T) {
	if got := Merge(nil, nil); got != nil {
		t.Errorf("Merge(nil, nil) = %v, want nil", got)
	}
}

func TestResolve_StandaloneAndManifestMerge(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawnlint.json", []byte(`{"profile": "strict"}`))

	mf := &manifest.Manifest{
		PawnKit: &manifest.PawnKitExtension{
			SchemaVersion: 1,
			Tool: map[string]map[string]any{
				"pawnlint": {"profile": "recommended", "extends": "base"},
			},
		},
	}

	got, err := Resolve(m, "/proj", "pawnlint", mf)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got["profile"] != "strict" {
		t.Errorf("profile = %v, want standalone to win", got["profile"])
	}

	if got["extends"] != "base" {
		t.Errorf("extends = %v, want manifest-only key preserved", got["extends"])
	}
}

func TestResolve_ManifestOnly(t *testing.T) {
	m := fsx.NewMem()

	mf := &manifest.Manifest{
		PawnKit: &manifest.PawnKitExtension{
			SchemaVersion: 1,
			Tool:          map[string]map[string]any{"pawnlint": {"profile": "recommended"}},
		},
	}

	got, err := Resolve(m, "/proj", "pawnlint", mf)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got["profile"] != "recommended" {
		t.Errorf("profile = %v", got["profile"])
	}
}

func TestResolve_Neither(t *testing.T) {
	m := fsx.NewMem()

	got, err := Resolve(m, "/proj", "pawnlint", &manifest.Manifest{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
