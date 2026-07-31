package lockfile

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMarshalSampctlDependenciesCreatesCompatibleLock(t *testing.T) {
	generated := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	packages := []Package{
		{
			Key: "github.com/owner/root", Constraint: ":v1", Name: "owner/root",
			Resolved: "v1", Commit: strings.Repeat("a", 40),
			Source: PackageSource{Type: SourceTypeGit, URL: "https://github.com/owner/root"},
			Kind:   KindDependency,
		},
		{
			Key: "github.com/owner/child", Name: "owner/child",
			Resolved: "HEAD", Commit: strings.Repeat("b", 40),
			Source: PackageSource{Type: SourceTypeGit, URL: "https://github.com/owner/child"},
			Kind:   KindDependency, Transitive: true,
			RequiredBy: []string{"github.com/owner/root"},
		},
	}

	content, err := MarshalSampctlDependencies(nil, packages, generated)
	if err != nil {
		t.Fatalf("MarshalSampctlDependencies: %v", err)
	}
	var document sampctlLock
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if document.Version != 1 || document.SampctlVersion != "1.14.1" ||
		document.Generated != "2026-07-31T12:00:00Z" {
		t.Fatalf("header = %+v", document)
	}
	child := document.Dependencies["github.com/owner/child"]
	if !child.Transitive || len(child.RequiredBy) != 1 ||
		child.Integrity != "commit:"+strings.Repeat("b", 40) {
		t.Fatalf("child = %+v", child)
	}
}

func TestMarshalSampctlDependenciesPreservesDataAndFiltersResources(t *testing.T) {
	checksum := "sha256:" + strings.Repeat("a", 64)
	source := []byte(`{
		"version": 1,
		"generated": "old",
		"sampctl_version": "1.14.1",
		"dependencies": {
			"github.com/owner/keep": {"commit":"` + strings.Repeat("a", 40) + `"},
			"github.com/owner/change": {"commit":"` + strings.Repeat("b", 40) + `"}
		},
		"runtime": {"version":"1.5.0"},
		"custom": {"keep":true},
		"pawnkit": {
			"schema_version": 1,
			"future": {"keep":true},
			"resources": [
				{"package":"github.com/owner/keep","resource":"keep","target":"linux-amd64","url":"https://example.com/keep","size":1,"checksum":"` + checksum + `","archive":"file","files":[{"source":"keep","destination":"plugins/keep","size":1,"checksum":"` + checksum + `"}]},
				{"package":"github.com/owner/change","resource":"change","target":"linux-amd64","url":"https://example.com/change","size":1,"checksum":"` + checksum + `","archive":"file","files":[{"source":"change","destination":"plugins/change","size":1,"checksum":"` + checksum + `"}]}
			]
		}
	}`)
	packages := []Package{
		writePackage("owner/keep", strings.Repeat("a", 40)),
		writePackage("owner/change", strings.Repeat("c", 40)),
	}

	content, err := MarshalSampctlDependencies(source, packages, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("MarshalSampctlDependencies: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var custom struct {
		Keep bool `json:"keep"`
	}
	if err := json.Unmarshal(document["custom"], &custom); err != nil ||
		!custom.Keep || len(document["runtime"]) == 0 {
		t.Fatalf("preserved data = %s", content)
	}
	var extension struct {
		Resources []ResolvedResource `json:"resources"`
		Future    struct {
			Keep bool `json:"keep"`
		} `json:"future"`
	}
	if err := json.Unmarshal(document["pawnkit"], &extension); err != nil {
		t.Fatalf("PawnKit extension: %v", err)
	}
	if len(extension.Resources) != 1 || extension.Resources[0].Package != "github.com/owner/keep" {
		t.Fatalf("resources = %+v", extension.Resources)
	}
	if !extension.Future.Keep {
		t.Fatalf("extension = %s", document["pawnkit"])
	}
}

func TestMarshalSampctlDependenciesRejectsDuplicateKeys(t *testing.T) {
	pkg := writePackage("owner/package", strings.Repeat("a", 40))
	_, err := MarshalSampctlDependencies(nil, []Package{pkg, pkg}, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "duplicate dependency key") {
		t.Fatalf("MarshalSampctlDependencies error = %v", err)
	}
}

func writePackage(name, commit string) Package {
	return Package{
		Key: "github.com/" + name, Name: name, Resolved: "HEAD", Commit: commit,
		Source: PackageSource{Type: SourceTypeGit, URL: "https://github.com/" + name},
		Kind:   KindDependency,
	}
}
