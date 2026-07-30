package toolchain

import (
	"errors"
	"testing"
)

type mapPathLookup map[string]string

func (lookup mapPathLookup) LookPath(name string) (string, error) {
	path, ok := lookup[name]
	if !ok {
		return "", errors.New("not found")
	}
	return path, nil
}

func TestFindCompilerUsesCandidateOrder(t *testing.T) {
	path, err := FindCompiler(mapPathLookup{
		"pawncc":        "/tools/pawncc",
		"openmp-pawncc": "/tools/openmp-pawncc",
	}, "openmp-pawncc", "pawncc")
	if err != nil {
		t.Fatalf("FindCompiler: %v", err)
	}
	if path != "/tools/openmp-pawncc" {
		t.Fatalf("path = %q", path)
	}
}

func TestFindCompilerReportsMissingPath(t *testing.T) {
	if _, err := FindCompiler(mapPathLookup{}, "pawncc"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}
