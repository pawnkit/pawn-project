package manifest

import (
	"os"
	"testing"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
)

func TestLoad_PawnKitSpecExample(t *testing.T) {
	content, err := os.ReadFile("testdata/pawnkit-spec/valid.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", content)

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, d := range res.Diagnostics {
		t.Errorf("unexpected diagnostic: [%s] %s", d.Code, d.Message)
	}

	if res.Manifest == nil {
		t.Fatal("Manifest is nil")
	}

	if res.Manifest.Entry != "gamemodes/example.pwn" {
		t.Errorf("Entry = %q", res.Manifest.Entry)
	}

	if res.Manifest.Preset != "openmp" {
		t.Errorf("Preset = %q", res.Manifest.Preset)
	}

	if len(res.Manifest.Dependencies) != 4 {
		t.Fatalf("Dependencies = %d, want 4", len(res.Manifest.Dependencies))
	}
	gitlab := res.Manifest.Dependencies[1]
	if gitlab.Site != "gitlab.com" || gitlab.Name() != "example/pawn-library" {
		t.Errorf("GitLab dependency parsed as %+v", gitlab)
	}

	if len(res.Manifest.Resources) != 1 ||
		res.Manifest.Resources[0].Name != "streamer-*-linux.tar.gz" ||
		len(res.Manifest.Resources[0].Includes) != 1 {
		t.Fatalf("Resources = %+v", res.Manifest.Resources)
	}

	streamer := res.Manifest.Dependencies[3]
	if streamer.Scheme != SchemePlugin || streamer.User != "samp-incognito" || streamer.Repo != "samp-streamer-plugin" {
		t.Errorf("streamer dependency parsed as %+v", streamer)
	}

	if streamer.RefKind != RefTag || streamer.Ref != "2.8.2" {
		t.Errorf("streamer ref = %s %q", streamer.RefKind, streamer.Ref)
	}

	if res.Manifest.PawnKit == nil || res.Manifest.PawnKit.SchemaVersion != 1 {
		t.Fatalf("PawnKit = %+v", res.Manifest.PawnKit)
	}

	if res.Manifest.PawnKit.Profile != "openmp" {
		t.Errorf("PawnKit.Profile = %q", res.Manifest.PawnKit.Profile)
	}
	if res.Manifest.Runtime == nil ||
		res.Manifest.Runtime.RCONPassword != "changeme" ||
		res.Manifest.Runtime.Hostname != "Example open.mp server" ||
		res.Manifest.Runtime.MaxPlayers != 50 {
		t.Errorf("Runtime = %+v", res.Manifest.Runtime)
	}

	if got := res.Manifest.EffectiveIncludePaths(); len(got) != 2 {
		t.Errorf("EffectiveIncludePaths = %v", got)
	}
}

func TestLoad_ValidatesResourceFieldsAndPaths(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{
		"resources": [
			{"platform": "linux"},
			{"name": "plugin.zip", "platform": "windows", "files": {"plugin.dll": "../plugins/plugin.dll"}}
		]
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var missing, traversal bool
	for _, d := range res.Diagnostics {
		missing = missing || d.Code == CodeMissingResourceField
		traversal = traversal || d.Code == CodePathTraversal
	}
	if !missing || !traversal {
		t.Fatalf("diagnostics = %+v", res.Diagnostics)
	}
}

func TestLoad_MinimalManifestIsValid(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(res.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v, want none", res.Diagnostics)
	}

	if res.Manifest == nil {
		t.Fatal("Manifest is nil")
	}
}

func TestParseDependencyHTTPSHost(t *testing.T) {
	dependency, err := ParseDependency("https://gitlab.com/example/library@main")
	if err != nil {
		t.Fatal(err)
	}
	if dependency.Site != "gitlab.com" || dependency.Name() != "example/library" ||
		dependency.RefKind != RefBranch || dependency.Ref != "main" ||
		dependency.RepositoryURL() != "https://gitlab.com/example/library" {
		t.Fatalf("dependency = %+v", dependency)
	}
}

func TestParseDependencyBranchWithSlash(t *testing.T) {
	dependency, err := ParseDependency("ScavengeSurvive/button@feat/pawnkit")
	if err != nil {
		t.Fatal(err)
	}
	if dependency.Name() != "ScavengeSurvive/button" || dependency.RefKind != RefBranch || dependency.Ref != "feat/pawnkit" {
		t.Fatalf("dependency = %#v", dependency)
	}
}

func TestParseDependencyRejectsUnsafeURL(t *testing.T) {
	for _, raw := range []string{
		"http://gitlab.com/example/library",
		"https://token@gitlab.com/example/library",
		"https://gitlab.com:8443/example/library",
		"https://gitlab.com/example/library/extra",
	} {
		if _, err := ParseDependency(raw); err == nil {
			t.Fatalf("ParseDependency(%q) succeeded", raw)
		}
	}
}

func TestLoad_RuntimePreservesExplicitFalse(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{
		"runtime": {
			"announce": false,
			"query": false,
			"rcon": false,
			"chatlogging": false
		}
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatal(err)
	}
	runtime := res.Manifest.Runtime
	if runtime == nil || runtime.Announce == nil || *runtime.Announce ||
		runtime.Query == nil || *runtime.Query ||
		runtime.RCON == nil || *runtime.RCON ||
		runtime.ChatLogging == nil || *runtime.ChatLogging {
		t.Fatalf("Runtime = %+v", runtime)
	}
}

func TestLoad_YAML(t *testing.T) {
	yaml := "entry: gamemodes/main.pwn\npreset: samp\ndependencies:\n  - pawn-lang/YSI-Includes\n"

	m := fsx.NewMem()
	m.AddFile("/proj/pawn.yaml", []byte(yaml))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(res.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", res.Diagnostics)
	}

	if res.Manifest.Entry != "gamemodes/main.pwn" || res.Manifest.Preset != "samp" {
		t.Errorf("manifest = %+v", res.Manifest)
	}

	if len(res.Manifest.Dependencies) != 1 || res.Manifest.Dependencies[0].Name() != "pawn-lang/YSI-Includes" {
		t.Errorf("Dependencies = %+v", res.Manifest.Dependencies)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"entry": "a.pwn",`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load returned Go error instead of a diagnostic: %v", err)
	}

	if res.Manifest != nil {
		t.Errorf("Manifest should be nil for unparsable content, got %+v", res.Manifest)
	}

	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != CodeParseError {
		t.Fatalf("Diagnostics = %+v, want single %s", res.Diagnostics, CodeParseError)
	}

	if err := res.Diagnostics[0].Validate(); err != nil {
		t.Errorf("diagnostic fails Validate: %v", err)
	}
}

func TestLoad_CorruptTopLevelType(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`["not", "an", "object"]`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if res.Manifest != nil {
		t.Error("Manifest should be nil for a non-object top level")
	}

	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Code != CodeInvalidType {
		t.Fatalf("Diagnostics = %+v", res.Diagnostics)
	}
}

func TestLoad_InvalidPreset(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"preset": "vice-city"}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeInvalidPreset)
}

func TestLoad_EmptyEntryAndOutput(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"entry": "", "output": ""}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeEmptyEntry)
	assertHasCode(t, res.Diagnostics, CodeEmptyOutput)
}

func TestLoad_InvalidDependencyString(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"dependencies": ["not-a-valid-dep", "user/repo"]}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeInvalidDependency)

	if len(res.Manifest.Dependencies) != 1 {
		t.Errorf("Dependencies = %+v, want the valid one to still parse", res.Manifest.Dependencies)
	}
}

func TestLoad_DependenciesWrongType(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"dependencies": "user/repo"}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeInvalidType)
}

func TestLoad_PawnKitSchemaVersionMissing(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"pawnkit": {"profile": "openmp"}}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeSchemaVersionMissing)
}

func TestLoad_PawnKitSchemaVersionUnsupported(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"pawnkit": {"schemaVersion": 99}}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeSchemaVersionInvalid)
}

func TestLoad_PawnKitUnknownField(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"pawnkit": {"schemaVersion": 1, "bogus": true}}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	d := assertHasCode(t, res.Diagnostics, CodeUnknownPawnKitField)
	if d.Severity != diagnostic.SeverityWarning {
		t.Errorf("severity = %v, want Warning", d.Severity)
	}
}

func TestLoad_DependencyOverrides(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{
		"pawnkit": {
			"schemaVersion": 1,
			"dependencyOverrides": {
				"Misiur/YSI-Includes": "pawn-lang/YSI-Includes@5.x"
			}
		}
	}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Diagnostics) != 0 || res.Manifest.PawnKit.DependencyOverrides["Misiur/YSI-Includes"] != "pawn-lang/YSI-Includes@5.x" {
		t.Fatalf("result = %#v", res)
	}
}

func TestLoad_RejectsInvalidDependencyOverrides(t *testing.T) {
	for name, overrides := range map[string]string{
		"versioned key":      `"owner/package:v1": "other/package"`,
		"scheme change":      `"plugin://owner/package": "other/package"`,
		"duplicate identity": `"owner/package": "one/package", "https://github.com/owner/package": "two/package"`,
	} {
		t.Run(name, func(t *testing.T) {
			m := fsx.NewMem()
			m.AddFile("/proj/pawn.json", []byte(`{"pawnkit":{"schemaVersion":1,"dependencyOverrides":{`+overrides+`}}}`))
			res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
			if err != nil {
				t.Fatal(err)
			}
			assertHasCode(t, res.Diagnostics, CodeInvalidDependency)
		})
	}
}

func TestLoad_InvalidProfilePattern(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"pawnkit": {"schemaVersion": 1, "profile": "Not_Valid!"}}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeInvalidProfile)
}

func TestLoad_IncludePathTraversal(t *testing.T) {
	cases := []string{"../outside", "/etc/passwd"}

	for _, p := range cases {
		m := fsx.NewMem()
		m.AddFile("/proj/pawn.json", []byte(`{"include_path": "`+p+`"}`))

		res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
		if err != nil {
			t.Fatalf("Load(%q): %v", p, err)
		}

		assertHasCode(t, res.Diagnostics, CodePathTraversal)
	}
}

func TestLoad_PawnKitIncludePathsTraversal(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"pawnkit": {"schemaVersion": 1, "includePaths": ["../../etc"]}}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodePathTraversal)
}

func TestLoad_BuildsMissingName(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"builds": [{"args": ["-d3"]}]}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeMissingBuildName)
}

func TestLoad_RuntimesMissingName(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.json", []byte(`{"runtimes": [{"port": 7777}]}`))

	res, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	assertHasCode(t, res.Diagnostics, CodeMissingRuntimeName)
}

func TestLoad_UnsupportedExtension(t *testing.T) {
	m := fsx.NewMem()
	m.AddFile("/proj/pawn.toml", []byte(``))

	_, err := Load(source.NewRegistry(), m, "/proj/pawn.toml")
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	m := fsx.NewMem()

	_, err := Load(source.NewRegistry(), m, "/proj/pawn.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
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
