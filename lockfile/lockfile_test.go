package lockfile

import (
	"os"
	"strings"
	"testing"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
)

var hex64 = strings.Repeat("a", 64)

func TestLoad_PawnKitSpecExample(t *testing.T) {
	content, err := os.ReadFile("testdata/pawnkit-spec/valid.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", content)

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, d := range res.Diagnostics {
		t.Errorf("unexpected diagnostic: [%s] %s", d.Code, d.Message)
	}

	if res.Lock == nil {
		t.Fatal("Lock is nil")
	}

	if len(res.Lock.Packages) != 3 {
		t.Fatalf("Packages = %d, want 3", len(res.Lock.Packages))
	}

	g := res.Lock.Graph()
	if roots := g.Roots(); len(roots) != 2 {
		t.Errorf("Roots = %v, want 2 (YSI-Includes and samp-streamer-plugin depend on nothing else pulling them, omp-stdlib is a dependency of YSI)", roots)
	}

	pkg, ok := res.Lock.ByName("pawn-lang/YSI-Includes")
	if !ok {
		t.Fatal("expected pawn-lang/YSI-Includes package")
	}

	if len(pkg.Dependencies) != 1 || pkg.Dependencies[0] != "openmultiplayer/omp-stdlib" {
		t.Errorf("Dependencies = %v", pkg.Dependencies)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{"schemaVersion": 1, "packages": [`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load returned Go error instead of diagnostic: %v", err)
	}

	if res.Lock != nil {
		t.Error("Lock should be nil for unparsable content")
	}

	assertHasCode(t, res.Diagnostics, CodeParseError)
}

func TestLoad_CorruptTopLevelType(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`"just a string"`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if res.Lock != nil {
		t.Error("Lock should be nil")
	}

	assertHasCode(t, res.Diagnostics, CodeInvalidType)
}

func TestLoad_SchemaVersionUnsupported(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{"schemaVersion": 2, "packages": []}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeSchemaVersionInvalid)
}

func TestLoad_MissingRequiredPackageFields(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [{"name": "a/b"}]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeMissingField)
}

func TestLoad_InvalidChecksumAndCommit(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [{
			"name": "a/b",
			"resolved": "a/b@main",
			"commit": "not-hex!",
			"source": {"type": "git", "url": "https://example.com/a/b"},
			"kind": "dependency",
			"checksum": "sha256:zzz"
		}]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeInvalidCommit)
	assertHasCode(t, res.Diagnostics, CodeInvalidChecksum)
}

func TestLoad_ArchiveWithoutChecksum(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [{
			"name": "a/b",
			"resolved": "a/b:1.0.0",
			"commit": "abc1234",
			"source": {"type": "archive", "url": "https://example.com/a.zip"},
			"kind": "plugin"
		}]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeMissingArchiveChecksum)
}

func TestLoad_DuplicatePackageName(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [
			{"name": "a/b", "resolved": "a/b@main", "commit": "abc1234", "source": {"type": "git", "url": "u"}, "kind": "dependency"},
			{"name": "a/b", "resolved": "a/b@dev", "commit": "def5678", "source": {"type": "git", "url": "u"}, "kind": "dependency"}
		]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeDuplicatePackage)
}

func TestLoad_UnknownDependencyEdge(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [
			{"name": "a/b", "resolved": "a/b@main", "commit": "abc1234", "source": {"type": "git", "url": "u"}, "kind": "dependency", "dependencies": ["x/y"]}
		]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeUnknownDependencyEdge)
}

func TestLoad_DependencyCycle(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [
			{"name": "a/b", "resolved": "a/b@main", "commit": "abc1234", "source": {"type": "git", "url": "u"}, "kind": "dependency", "dependencies": ["c/d"]},
			{"name": "c/d", "resolved": "c/d@main", "commit": "def5678", "source": {"type": "git", "url": "u"}, "kind": "dependency", "dependencies": ["a/b"]}
		]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeDependencyCycle)
}

func TestLoad_PlatformArtifactPathTraversal(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.lock", []byte(`{
		"schemaVersion": 1,
		"packages": [{
			"name": "a/b",
			"resolved": "a/b:1.0.0",
			"commit": "abc1234",
			"source": {"type": "archive", "url": "https://example.com/a.zip"},
			"kind": "plugin",
			"checksum": "sha256:`+hex64+`",
			"platformArtifacts": [{"platform": "linux-x86_64", "path": "../../etc/passwd"}]
		}]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.lock")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodePathTraversal)
}

func assertHasCode(t *testing.T, diags []diagnostic.Diagnostic, code string) diagnostic.Diagnostic {
	t.Helper()

	for _, d := range diags {
		if d.Code == code {
			if err := d.Validate(); err != nil {
				t.Errorf("diagnostic %s fails Validate: %v", code, err)
			}

			return d
		}
	}

	t.Fatalf("no diagnostic with code %s found in %+v", code, diags)

	return diagnostic.Diagnostic{}
}
