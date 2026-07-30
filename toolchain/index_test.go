package toolchain

import (
	"strings"
	"testing"

	"github.com/pawnkit/pawnkit-core/hash"
)

func compilerIndexJSON(extra string) string {
	return `{
		"schemaVersion":1,
		"id":"test",
		"generatedAt":"2026-07-30T18:00:00Z",
		"artifacts":[{
			"vendor":"pawn-lang",
			"version":"3.10.10",
			"profiles":["samp-037"],
			"target":"linux-amd64",
			"source":{
				"repository":"pawn-lang/compiler",
				"tag":"v3.10.10",
				"commit":"289cfeb1268f833ae1796debbab1e464306296ca"
			},
			"archive":{
				"url":"https://github.com/pawn-lang/compiler/releases/download/v3.10.10/compiler.tar.gz",
				"format":"tar.gz",
				"size":123,
				"checksum":"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
			},
			"executable":{
				"path":"compiler/bin/pawncc",
				"architecture":"386",
				"checksum":"sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
			}
		}]` + extra + `}`
}

func loadCompilerIndex(t *testing.T, document string) (Index, error) {
	t.Helper()
	return LoadIndex(strings.NewReader(document), hash.Content([]byte(document)))
}

func TestLoadIndexAndSelect(t *testing.T) {
	index, err := loadCompilerIndex(t, compilerIndexJSON(""))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := index.Select(VendorPawnLang, "3.10.10", "linux-amd64")
	if err != nil || artifact.Executable.Path != "compiler/bin/pawncc" {
		t.Fatalf("artifact = %+v, error = %v", artifact, err)
	}
	if _, err := index.Select(VendorPawnLang, "3.10.10", "windows-amd64"); err == nil {
		t.Fatal("missing target selected")
	}
}

func TestLoadIndexRejectsOuterChecksumMismatch(t *testing.T) {
	_, err := LoadIndex(
		strings.NewReader(compilerIndexJSON("")),
		"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	)
	if err == nil {
		t.Fatal("checksum mismatch accepted")
	}
}

func TestLoadIndexRejectsUnknownFields(t *testing.T) {
	if _, err := loadCompilerIndex(t, compilerIndexJSON(`,"unknown":true`)); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestLoadIndexRejectsDuplicateCoordinates(t *testing.T) {
	document := strings.Replace(
		compilerIndexJSON(""),
		`}]}`,
		`}, {
			"vendor":"pawn-lang",
			"version":"3.10.10",
			"profiles":["samp-037"],
			"target":"linux-amd64",
			"source":{"repository":"pawn-lang/compiler","tag":"v3.10.10","commit":"289cfeb1268f833ae1796debbab1e464306296ca"},
			"archive":{"url":"https://example.test/compiler","format":"raw","size":1,"checksum":"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
			"executable":{"path":"pawncc","architecture":"386","checksum":"sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}
		}]}`,
		1,
	)
	if _, err := loadCompilerIndex(t, document); err == nil {
		t.Fatal("duplicate coordinate accepted")
	}
}

func TestLoadIndexRejectsUnsafeExecutablePath(t *testing.T) {
	document := strings.Replace(compilerIndexJSON(""), "compiler/bin/pawncc", "../pawncc", 1)
	if _, err := loadCompilerIndex(t, document); err == nil {
		t.Fatal("unsafe executable path accepted")
	}
}
