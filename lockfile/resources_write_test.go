package lockfile

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalSampctlResourcesPreservesExistingData(t *testing.T) {
	source := []byte(`{
		"version": 1,
		"generated": "2026-07-31T00:00:00Z",
		"sampctl_version": "1.14.1",
		"dependencies": {
			"plugin://owner/plugin": {
				"constraint": "plugin://owner/plugin:v1",
				"resolved": "v1",
				"commit": "abcdef0",
				"user": "owner",
				"repo": "plugin",
				"scheme": "plugin"
			}
		},
		"custom": {"keep": true},
		"pawnkit": {"future": {"keep": true}}
	}`)
	lock := &Lock{SchemaVersion: 1, Packages: []Package{{
		Name: "owner/plugin", Commit: "abcdef0", Kind: KindPlugin,
		Source: PackageSource{Type: SourceTypeGit, URL: "https://github.com/owner/plugin"},
	}}}
	resource := validWriteResource()

	updated, err := MarshalSampctlResources(source, lock, []ResolvedResource{resource})
	if err != nil {
		t.Fatalf("MarshalSampctlResources: %v", err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(updated, &document); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	var dependencies map[string]struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(document["dependencies"], &dependencies); err != nil {
		t.Fatalf("dependencies: %v", err)
	}
	if dependencies["plugin://owner/plugin"].Commit != "abcdef0" {
		t.Fatalf("dependencies changed: %s", document["dependencies"])
	}
	var custom struct {
		Keep bool `json:"keep"`
	}
	if err := json.Unmarshal(document["custom"], &custom); err != nil || !custom.Keep {
		t.Fatalf("custom field changed: %s", document["custom"])
	}
	var extension map[string]json.RawMessage
	if err := json.Unmarshal(document["pawnkit"], &extension); err != nil {
		t.Fatalf("PawnKit extension: %v", err)
	}
	var future struct {
		Keep bool `json:"keep"`
	}
	if err := json.Unmarshal(extension["future"], &future); err != nil ||
		!future.Keep ||
		len(extension["resources"]) == 0 {
		t.Fatalf("PawnKit extension = %s", document["pawnkit"])
	}
}

func TestMarshalSampctlResourcesRejectsLegacyShape(t *testing.T) {
	_, err := MarshalSampctlResources(
		[]byte(`{"schemaVersion":1,"packages":[]}`),
		&Lock{SchemaVersion: 1},
		nil,
	)
	if err == nil {
		t.Fatal("MarshalSampctlResources succeeded")
	}
}

func TestMarshalSampctlResourcesValidatesUpdate(t *testing.T) {
	source := []byte(`{"version":1,"dependencies":{}}`)
	lock := &Lock{SchemaVersion: 1}
	_, err := MarshalSampctlResources(source, lock, []ResolvedResource{validWriteResource()})
	if err == nil || !strings.Contains(err.Error(), "not in dependencies") {
		t.Fatalf("MarshalSampctlResources error = %v", err)
	}
}

func TestMarshalSampctlResourcesAcceptsExactDependencyKey(t *testing.T) {
	source := []byte(`{
		"version": 1,
		"dependencies": {
			"github.com/owner/plugin": {
				"resolved": "v1",
				"commit": "abcdef0",
				"user": "owner",
				"repo": "plugin"
			}
		}
	}`)
	lock := &Lock{SchemaVersion: 1, Packages: []Package{{
		Key: "github.com/owner/plugin", Name: "owner/plugin",
		Commit: "abcdef0", Kind: KindDependency,
		Source: PackageSource{Type: SourceTypeGit, URL: "https://github.com/owner/plugin"},
	}}}
	resource := validWriteResource()
	resource.Package = "github.com/owner/plugin"

	if _, err := MarshalSampctlResources(source, lock, []ResolvedResource{resource}); err != nil {
		t.Fatalf("MarshalSampctlResources: %v", err)
	}
}

func validWriteResource() ResolvedResource {
	checksum := "sha256:" + strings.Repeat("a", 64)
	return ResolvedResource{
		Package: "plugin://owner/plugin", Resource: "plugin",
		Target: "linux-amd64", URL: "https://example.com/plugin.so",
		Size: 1, Checksum: checksum, Archive: "file",
		Files: []ResolvedResourceFile{{
			Source: "plugin.so", Destination: "plugins/plugin.so",
			Size: 1, Checksum: checksum,
		}},
	}
}
