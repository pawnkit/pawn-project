// Package project loads a complete, immutable Pawn project view.
package project

import (
	"fmt"
	"slices"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fingerprint"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/include"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/paths"
	"github.com/pawnkit/pawn-project/pathutil"
	"github.com/pawnkit/pawn-project/profile"
	"github.com/pawnkit/pawn-project/workspace"
)

// Options configures [Load].
type Options struct {
	// Profile contains profile, build, and runtime overrides.
	Profile profile.Options
}

// Project is the resolved view of a Pawn workspace.
type Project struct {
	root         string
	workspace    workspace.Root
	manifest     *manifest.Manifest
	manifestDiag []diagnostic.Diagnostic
	lock         *lockfile.Lock
	lockDiag     []diagnostic.Diagnostic
	selection    profile.Selection
	resolved     paths.Resolved
	includes     *include.Resolver
}

// Load discovers and resolves the project containing start. Content issues
// are returned as diagnostics; environment failures return an error.
func Load(reg *source.Registry, fsys fsx.FS, start string, opts Options) (*Project, error) {
	root, err := workspace.FindRoot(fsys, start)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}

	manifestRes, err := manifest.Load(reg, fsys, root.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("project: loading manifest: %w", err)
	}

	m := manifestRes.Manifest
	if m == nil {
		m = &manifest.Manifest{}
	}

	selection, err := profile.Select(m, opts.Profile)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}

	resolved, err := paths.Resolve(root.Dir, m)
	if err != nil {
		return nil, fmt.Errorf("project: resolving paths: %w", err)
	}
	resolved.IncludeRoots, err = projectIncludeRoots(fsys, reg, root.Dir, resolved.Entry, selection, resolved.IncludeRoots)
	if err != nil {
		return nil, fmt.Errorf("project: resolving includes: %w", err)
	}

	var quotedRoots []string
	if resolved.Entry != "" {
		quotedRoots = append(quotedRoots, pathutil.Dir(resolved.Entry))
	}

	p := &Project{
		root:         root.Dir,
		workspace:    root,
		manifest:     manifestRes.Manifest,
		manifestDiag: manifestRes.Diagnostics,
		selection:    selection,
		resolved:     resolved,
		includes:     include.NewWithQuotedRoots(fsys, resolved.IncludeRoots, quotedRoots),
	}

	lockRelPath := m.PawnKit.LockfilePath()

	lockPath, err := pathutil.SafeJoin(root.Dir, lockRelPath)
	if err == nil && fsx.IsFile(fsys, lockPath) {
		lockRes, err := lockfile.Load(reg, fsys, lockPath)
		if err != nil {
			return nil, fmt.Errorf("project: loading lockfile: %w", err)
		}

		p.lock = lockRes.Lock
		p.lockDiag = lockRes.Diagnostics
	}

	return p, nil
}

func projectIncludeRoots(
	fsys fsx.FS,
	reg *source.Registry,
	root string,
	entry string,
	selection profile.Selection,
	declared []string,
) ([]string, error) {
	var roots []string
	if selection.Build != nil {
		for _, rel := range selection.Build.Includes {
			path, err := pathutil.SafeJoin(root, rel)
			if err != nil {
				return nil, err
			}
			roots = appendUnique(roots, path)
		}
	}
	if entry != "" {
		roots = appendUnique(roots, pathutil.Dir(entry))
	}
	roots = appendUnique(roots, root)
	for _, path := range declared {
		roots = appendUnique(roots, path)
	}
	for _, path := range installedDependencyRoots(fsys, reg, root) {
		roots = appendUnique(roots, path)
	}
	return roots, nil
}

func installedDependencyRoots(fsys fsx.FS, reg *source.Registry, root string) []string {
	directory := pathutil.Join(root, "dependencies")
	entries, err := fsys.ReadDir(directory)
	if err != nil {
		return nil
	}
	var roots []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := pathutil.Join(directory, entry.Name())
		if entry.Name() == ".resources" {
			resources, readErr := fsys.ReadDir(path)
			if readErr != nil {
				continue
			}
			for _, resource := range resources {
				if resource.IsDir() {
					roots = appendUnique(roots, pathutil.Join(path, resource.Name()))
				}
			}
			continue
		}
		roots = append(roots, dependencyPackageRoots(fsys, reg, path)...)
	}
	return roots
}

func dependencyPackageRoots(fsys fsx.FS, reg *source.Registry, root string) []string {
	manifestPath := pathutil.Join(root, "pawn.json")
	if !fsx.IsFile(fsys, manifestPath) {
		manifestPath = pathutil.Join(root, "pawn.yaml")
	}
	if !fsx.IsFile(fsys, manifestPath) {
		return dependencyFallbackRoots(fsys, root)
	}
	result, err := manifest.Load(reg, fsys, manifestPath)
	if err != nil || result.Manifest == nil {
		return dependencyFallbackRoots(fsys, root)
	}
	selection, err := profile.Select(result.Manifest, profile.Options{})
	if err != nil {
		return []string{root}
	}
	var relative []string
	if selection.Build != nil {
		relative = append(relative, selection.Build.Includes...)
	}
	relative = append(relative, result.Manifest.EffectiveIncludePaths()...)
	if len(relative) == 0 {
		return []string{root}
	}
	var roots []string
	for _, rel := range relative {
		path, joinErr := pathutil.SafeJoin(root, rel)
		if joinErr == nil {
			roots = appendUnique(roots, path)
		}
	}
	return roots
}

func dependencyFallbackRoots(fsys fsx.FS, root string) []string {
	include := pathutil.Join(root, "include")
	if fsx.IsDir(fsys, include) {
		return []string{include}
	}
	return []string{root}
}

func appendUnique(items []string, value string) []string {
	if slices.Contains(items, value) {
		return items
	}
	return append(items, value)
}

func (p *Project) Root() string                       { return p.root }
func (p *Project) Workspace() workspace.Root          { return p.workspace }
func (p *Project) Manifest() *manifest.Manifest       { return p.manifest }
func (p *Project) Lockfile() *lockfile.Lock           { return p.lock }
func (p *Project) Selection() profile.Selection       { return p.selection }
func (p *Project) Paths() paths.Resolved              { return p.resolved }
func (p *Project) IncludeResolver() *include.Resolver { return p.includes }

// Diagnostics returns every diagnostic accumulated while loading the
// manifest and lockfile, in that order.
func (p *Project) Diagnostics() []diagnostic.Diagnostic {
	out := make([]diagnostic.Diagnostic, 0, len(p.manifestDiag)+len(p.lockDiag))
	out = append(out, p.manifestDiag...)
	out = append(out, p.lockDiag...)

	return out
}

// Fingerprint computes a stable cache key for the project's current
// resolved state; see package fingerprint.
func (p *Project) Fingerprint() (string, error) {
	return fingerprint.Compute(fingerprint.Inputs{
		Manifest:  p.manifest,
		Lock:      p.lock,
		ProfileID: p.selection.ProfileID,
	})
}
