package fingerprint

import (
	"testing"

	"github.com/pawnkit/pawn-project/manifest"
)

func TestCompute_DeterministicAndSensitive(t *testing.T) {
	a := Inputs{Manifest: &manifest.Manifest{Entry: "a.pwn"}, ProfileID: "openmp"}
	b := Inputs{Manifest: &manifest.Manifest{Entry: "a.pwn"}, ProfileID: "openmp"}
	c := Inputs{Manifest: &manifest.Manifest{Entry: "b.pwn"}, ProfileID: "openmp"}

	fa, err := Compute(a)
	if err != nil {
		t.Fatalf("Compute(a): %v", err)
	}

	fb, err := Compute(b)
	if err != nil {
		t.Fatalf("Compute(b): %v", err)
	}

	fc, err := Compute(c)
	if err != nil {
		t.Fatalf("Compute(c): %v", err)
	}

	if fa != fb {
		t.Errorf("equal inputs produced different fingerprints: %q vs %q", fa, fb)
	}

	if fa == fc {
		t.Errorf("different inputs produced the same fingerprint: %q", fa)
	}
}

func TestCompute_ProfileIDAffectsFingerprint(t *testing.T) {
	base := &manifest.Manifest{Entry: "a.pwn"}

	f1, err := Compute(Inputs{Manifest: base, ProfileID: "openmp"})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	f2, err := Compute(Inputs{Manifest: base, ProfileID: "samp-037"})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if f1 == f2 {
		t.Error("different profile IDs should produce different fingerprints")
	}
}
