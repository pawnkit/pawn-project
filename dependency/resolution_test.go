package dependency

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

const (
	commitA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	commitB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	commitD = "dddddddddddddddddddddddddddddddddddddddd"
)

type graphRevisionProvider struct {
	revisions map[string]Revision
	locked    map[string]bool
}

func (p *graphRevisionProvider) Resolve(
	_ context.Context,
	dependency manifest.Dependency,
	locked *lockfile.Package,
) (Revision, error) {
	key := dependencyKey(dependency)
	p.locked[key] = locked != nil
	revision, ok := p.revisions[key]
	if !ok {
		return Revision{}, fmt.Errorf("missing fixture %s", key)
	}
	return revision, nil
}

func TestGraphResolverBuildsTransitiveGraph(t *testing.T) {
	root := &manifest.Manifest{
		Dependencies: []manifest.Dependency{mustDependency(t, "owner/a:v1")},
		DevDependencies: []manifest.Dependency{
			mustDependency(t, "owner/dev#"+commitD),
		},
	}
	provider := &graphRevisionProvider{
		revisions: map[string]Revision{
			"github.com/owner/a": {
				Commit: commitA, Resolved: "v1",
				Manifest: manifest.Manifest{Dependencies: []manifest.Dependency{
					mustDependency(t, "owner/b@main"),
				}},
			},
			"github.com/owner/b":   {Commit: commitB, Resolved: "main"},
			"github.com/owner/dev": {Commit: commitD, Resolved: commitD[:8]},
		},
		locked: map[string]bool{},
	}

	packages, err := NewGraphResolver(provider).Resolve(context.Background(), root, nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(packages) != 3 {
		t.Fatalf("packages = %#v", packages)
	}
	a := packageByKey(t, packages, "github.com/owner/a")
	if a.Constraint != ":v1" || a.Transitive ||
		len(a.Dependencies) != 1 || a.Dependencies[0] != "owner/b" {
		t.Fatalf("a = %#v", a)
	}
	b := packageByKey(t, packages, "github.com/owner/b")
	if b.Constraint != "@main" || b.Branch != "main" || !b.Transitive ||
		len(b.RequiredBy) != 1 || b.RequiredBy[0] != a.Key {
		t.Fatalf("b = %#v", b)
	}
	dev := packageByKey(t, packages, "github.com/owner/dev")
	if dev.Kind != lockfile.KindDevDependency || dev.Integrity != "commit:"+commitD {
		t.Fatalf("dev = %#v", dev)
	}
}

func TestGraphResolverPrefersDirectConstraintOverUnqualifiedTransitive(t *testing.T) {
	root := &manifest.Manifest{Dependencies: []manifest.Dependency{
		mustDependency(t, "owner/a#"+commitA),
		mustDependency(t, "owner/b:v1"),
	}}
	provider := &graphRevisionProvider{
		revisions: map[string]Revision{
			"github.com/owner/a": {Commit: commitA, Resolved: commitA[:8]},
			"github.com/owner/b": {
				Commit: commitB, Resolved: "v1",
				Manifest: manifest.Manifest{Dependencies: []manifest.Dependency{
					mustDependency(t, "owner/a"),
				}},
			},
		},
		locked: map[string]bool{},
	}

	packages, err := NewGraphResolver(provider).Resolve(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := packageByKey(t, packages, "github.com/owner/a")
	if a.Constraint != "#"+commitA || a.Transitive || len(a.RequiredBy) != 1 {
		t.Fatalf("a = %#v", a)
	}
}

func TestGraphResolverPassesMatchingLockEntry(t *testing.T) {
	dependency := mustDependency(t, "owner/a:v1")
	root := &manifest.Manifest{Dependencies: []manifest.Dependency{dependency}}
	provider := &graphRevisionProvider{
		revisions: map[string]Revision{
			"github.com/owner/a": {Commit: commitA, Resolved: "v1"},
		},
		locked: map[string]bool{},
	}
	existing := &lockfile.Lock{Packages: []lockfile.Package{{
		Key: "github.com/owner/a", Constraint: ":v1", Commit: commitA,
	}}}

	if _, err := NewGraphResolver(provider).Resolve(context.Background(), root, existing); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !provider.locked["github.com/owner/a"] {
		t.Fatal("matching lock entry was not passed to provider")
	}
}

func TestGraphResolverUpdateDoesNotReuseLockEntry(t *testing.T) {
	dependency := mustDependency(t, "owner/package@main")
	root := &manifest.Manifest{Dependencies: []manifest.Dependency{dependency}}
	existing := &lockfile.Lock{Packages: []lockfile.Package{{
		Key: "github.com/owner/package", Constraint: "@main",
		Commit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}}
	provider := &graphRevisionProvider{
		revisions: map[string]Revision{
			"github.com/owner/package": {
				Commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Resolved: "main",
			},
		},
		locked: map[string]bool{},
	}

	packages, err := NewGraphResolver(provider).ResolveWithOptions(
		context.Background(), root, existing, ResolveOptions{Update: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if provider.locked["github.com/owner/package"] {
		t.Fatal("update reused the locked revision")
	}
	if len(packages) != 1 || packages[0].Commit != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("packages = %+v", packages)
	}
}

func TestGraphResolverCollapsesProviderAliases(t *testing.T) {
	root := &manifest.Manifest{Dependencies: []manifest.Dependency{
		mustDependency(t, "old-owner/package"),
		mustDependency(t, "new-owner/package"),
	}}
	provider := &graphRevisionProvider{
		revisions: map[string]Revision{
			"github.com/old-owner/package": {
				Commit: commitA, Resolved: "HEAD", CanonicalName: "new-owner/package",
			},
		},
		locked: map[string]bool{},
	}

	packages, err := NewGraphResolver(provider).Resolve(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Name != "new-owner/package" ||
		packages[0].Source.URL != "https://github.com/new-owner/package" {
		t.Fatalf("packages = %+v", packages)
	}
}

func TestGraphResolverKeepsProviderHostsDistinct(t *testing.T) {
	github := mustDependency(t, "owner/package")
	gitlab := mustDependency(t, "https://gitlab.com/owner/package")
	root := &manifest.Manifest{Dependencies: []manifest.Dependency{github, gitlab}}
	provider := &graphRevisionProvider{
		revisions: map[string]Revision{
			"github.com/owner/package": {Commit: commitA, Resolved: "HEAD"},
			"gitlab.com/owner/package": {Commit: commitB, Resolved: "HEAD"},
		},
		locked: map[string]bool{},
	}

	packages, err := NewGraphResolver(provider).Resolve(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 || packages[0].Key == packages[1].Key {
		t.Fatalf("packages = %+v", packages)
	}
}

func TestLockNeedsResolution(t *testing.T) {
	dependency := mustDependency(t, "owner/package:v1")
	root := &manifest.Manifest{Dependencies: []manifest.Dependency{dependency}}
	matching := &lockfile.Lock{Packages: []lockfile.Package{{
		Key: "github.com/owner/package", Constraint: ":v1", Kind: lockfile.KindDependency,
	}}}
	if LockNeedsResolution(root, matching) {
		t.Fatal("matching lock needs resolution")
	}

	tests := map[string]*lockfile.Lock{
		"missing lock":    nil,
		"missing package": {Packages: nil},
		"changed constraint": {Packages: []lockfile.Package{{
			Key: "github.com/owner/package", Constraint: ":v2", Kind: lockfile.KindDependency,
		}}},
		"transitive direct package": {Packages: []lockfile.Package{{
			Key: "github.com/owner/package", Constraint: ":v1", Kind: lockfile.KindDependency, Transitive: true,
		}}},
		"removed direct package": {Packages: []lockfile.Package{
			matching.Packages[0],
			{Key: "github.com/owner/removed", Constraint: ":v1", Kind: lockfile.KindDependency},
		}},
	}
	for name, lock := range tests {
		t.Run(name, func(t *testing.T) {
			if !LockNeedsResolution(root, lock) {
				t.Fatal("lock does not need resolution")
			}
		})
	}
}

func TestLockNeedsResolutionChecksDependencyKind(t *testing.T) {
	dependency := mustDependency(t, "owner/tool:v1")
	root := &manifest.Manifest{DevDependencies: []manifest.Dependency{dependency}}
	lock := &lockfile.Lock{Packages: []lockfile.Package{{
		Key: "github.com/owner/tool", Constraint: ":v1", Kind: lockfile.KindDependency,
	}}}
	if !LockNeedsResolution(root, lock) {
		t.Fatal("dependency kind change was not detected")
	}
}

func TestGraphResolverRejectsConflictsAndCycles(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		root := &manifest.Manifest{Dependencies: []manifest.Dependency{
			mustDependency(t, "owner/a:v1"),
			mustDependency(t, "owner/b:v1"),
		}}
		provider := &graphRevisionProvider{
			revisions: map[string]Revision{
				"github.com/owner/a": {Commit: commitA, Resolved: "v1"},
				"github.com/owner/b": {
					Commit: commitB, Resolved: "v1",
					Manifest: manifest.Manifest{Dependencies: []manifest.Dependency{
						mustDependency(t, "owner/a:v2"),
					}},
				},
			},
			locked: map[string]bool{},
		}
		_, err := NewGraphResolver(provider).Resolve(context.Background(), root, nil)
		if err == nil || !strings.Contains(err.Error(), "conflicting constraints") {
			t.Fatalf("Resolve error = %v", err)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		root := &manifest.Manifest{Dependencies: []manifest.Dependency{
			mustDependency(t, "owner/a:v1"),
		}}
		provider := &graphRevisionProvider{
			revisions: map[string]Revision{
				"github.com/owner/a": {
					Commit: commitA, Resolved: "v1",
					Manifest: manifest.Manifest{Dependencies: []manifest.Dependency{
						mustDependency(t, "owner/b:v1"),
					}},
				},
				"github.com/owner/b": {
					Commit: commitB, Resolved: "v1",
					Manifest: manifest.Manifest{Dependencies: []manifest.Dependency{
						mustDependency(t, "owner/a:v1"),
					}},
				},
			},
			locked: map[string]bool{},
		}
		_, err := NewGraphResolver(provider).Resolve(context.Background(), root, nil)
		if err == nil || !strings.Contains(err.Error(), "cycle detected") {
			t.Fatalf("Resolve error = %v", err)
		}
	})
}

func mustDependency(t *testing.T, raw string) manifest.Dependency {
	t.Helper()
	dependency, err := manifest.ParseDependency(raw)
	if err != nil {
		t.Fatalf("ParseDependency(%q): %v", raw, err)
	}
	return dependency
}

func packageByKey(t *testing.T, packages []lockfile.Package, key string) lockfile.Package {
	t.Helper()
	for _, pkg := range packages {
		if pkg.Key == key {
			return pkg
		}
	}
	t.Fatalf("package %q not found in %#v", key, packages)
	return lockfile.Package{}
}
