package dependency

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
)

const (
	maxResolutionPackages = 1024
	maxResolutionDepth    = 64
)

var fullCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Revision is one provider-resolved package revision and manifest.
type Revision struct {
	Commit        string
	Resolved      string
	CanonicalSite string
	CanonicalName string
	SourceURL     string
	Manifest      manifest.Manifest
}

// RevisionProvider resolves a manifest dependency to an exact revision.
type RevisionProvider interface {
	Resolve(context.Context, manifest.Dependency, *lockfile.Package) (Revision, error)
}

// GraphResolver builds one deterministic dependency graph.
type GraphResolver struct {
	provider RevisionProvider
}

// ResolveOptions controls dependency graph resolution.
type ResolveOptions struct {
	Update bool
}

// NewGraphResolver creates a dependency graph resolver.
func NewGraphResolver(provider RevisionProvider) *GraphResolver {
	return &GraphResolver{provider: provider}
}

// LockNeedsResolution reports whether direct dependencies changed.
func LockNeedsResolution(root *manifest.Manifest, existing *lockfile.Lock) bool {
	if root == nil || existing == nil {
		return true
	}

	expected := make(map[string]resolutionRequest, len(root.Dependencies)+len(root.DevDependencies))
	for _, request := range rootResolutionRequests(root) {
		expected[dependencyKey(request.dependency)] = request
	}

	found := make(map[string]bool, len(expected))
	for _, pkg := range existing.Packages {
		request, direct := expected[pkg.Key]
		if !pkg.Transitive && !direct {
			return true
		}
		if !direct {
			continue
		}
		if pkg.Transitive || pkg.Constraint != dependencyConstraint(request.dependency) ||
			pkg.Kind != dependencyKind(request.dependency, request.dev) {
			return true
		}
		found[pkg.Key] = true
	}

	return len(found) != len(expected)
}

type resolutionRequest struct {
	dependency manifest.Dependency
	parent     string
	ancestry   []string
	direct     bool
	dev        bool
}

// Resolve selects exact revisions for a project manifest.
func (r *GraphResolver) Resolve(
	ctx context.Context,
	root *manifest.Manifest,
	existing *lockfile.Lock,
) ([]lockfile.Package, error) {
	return r.ResolveWithOptions(ctx, root, existing, ResolveOptions{})
}

// ResolveWithOptions selects exact revisions with explicit update behavior.
func (r *GraphResolver) ResolveWithOptions( //nolint:gocyclo,funlen // Graph traversal validates each state transition.
	ctx context.Context,
	root *manifest.Manifest,
	existing *lockfile.Lock,
	options ResolveOptions,
) ([]lockfile.Package, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, errors.New("dependency: project manifest is required")
	}
	if r.provider == nil {
		return nil, errors.New("dependency: revision provider is required")
	}

	locked := lockedPackages(existing)
	overrides, err := dependencyOverrides(root)
	if err != nil {
		return nil, err
	}
	queue := rootResolutionRequests(root)
	packages := make(map[string]*lockfile.Package)
	constraints := make(map[string]string)
	canonicalKeys := make(map[string]string)

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request := queue[0]
		queue = queue[1:]
		if !request.direct {
			request.dependency, err = applyDependencyOverride(request.dependency, overrides)
			if err != nil {
				return nil, err
			}
		}
		key := dependencyKey(request.dependency)
		constraint := dependencyConstraint(request.dependency)

		if len(request.ancestry) > maxResolutionDepth {
			return nil, fmt.Errorf("dependency: graph exceeds %d levels at %s", maxResolutionDepth, key)
		}
		if canonicalKey, ok := canonicalKeys[dependencyCanonicalID(request.dependency)]; ok {
			if previous := constraints[canonicalKey]; constraintsConflict(
				previous, constraint, packages[canonicalKey], request,
			) {
				return nil, conflictingConstraintsError(canonicalKey, previous, constraint)
			}
			mergeResolutionEdge(packages, canonicalKey, request)
			continue
		}
		if previous, ok := constraints[key]; ok {
			if constraintsConflict(previous, constraint, packages[key], request) {
				return nil, conflictingConstraintsError(key, previous, constraint)
			}
			mergeResolutionEdge(packages, key, request)
			continue
		}
		if len(packages) >= maxResolutionPackages {
			return nil, fmt.Errorf("dependency: graph exceeds %d packages", maxResolutionPackages)
		}

		reusable := reusableLockedPackage(locked, key, constraint, options.Update)
		revision, err := r.provider.Resolve(ctx, request.dependency, reusable)
		if err != nil {
			return nil, fmt.Errorf("dependency: resolving %s: %w", key, err)
		}
		if !fullCommitPattern.MatchString(revision.Commit) {
			return nil, fmt.Errorf("dependency: %s resolved invalid commit %q", key, revision.Commit)
		}
		canonicalSite := revision.CanonicalSite
		if canonicalSite == "" {
			canonicalSite = request.dependency.Site
		}
		if canonicalSite == "" {
			canonicalSite = "github.com"
		}
		canonicalName := revision.CanonicalName
		if canonicalName == "" {
			canonicalName = request.dependency.Name()
		}
		canonicalID := canonicalSite + "/" + canonicalName
		if revision.SourceURL == "" {
			revision.SourceURL = "https://" + canonicalID
		}
		if canonicalKey, ok := canonicalKeys[canonicalID]; ok {
			previous := constraints[canonicalKey]
			if constraintsConflict(previous, constraint, packages[canonicalKey], request) {
				return nil, conflictingConstraintsError(canonicalKey, previous, constraint)
			}
			mergeResolutionEdge(packages, canonicalKey, request)
			continue
		}

		pkg := &lockfile.Package{
			Key: key, Constraint: constraint, Name: canonicalName,
			Resolved: revision.Resolved, Commit: revision.Commit,
			Source:     lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: revision.SourceURL},
			Integrity:  "commit:" + revision.Commit,
			Kind:       dependencyKind(request.dependency, request.dev),
			Branch:     dependencyBranch(request.dependency),
			Transitive: !request.direct,
		}
		if request.parent != "" {
			pkg.RequiredBy = []string{request.parent}
		}
		packages[key] = pkg
		constraints[key] = constraint
		canonicalKeys[canonicalID] = key
		canonicalKeys[dependencyCanonicalID(request.dependency)] = key
		if request.parent != "" {
			addForwardDependency(packages, request.parent, pkg.Name)
		}

		ancestry := append(append([]string(nil), request.ancestry...), key)
		for _, child := range revision.Manifest.Dependencies {
			queue = append(queue, resolutionRequest{
				dependency: child,
				parent:     key,
				ancestry:   ancestry,
			})
		}
	}

	return sortedResolvedPackages(packages), nil
}

func dependencyOverrides(root *manifest.Manifest) (map[string]manifest.Dependency, error) {
	if root.PawnKit == nil || len(root.PawnKit.DependencyOverrides) == 0 {
		return nil, nil
	}
	overrides := make(map[string]manifest.Dependency, len(root.PawnKit.DependencyOverrides))
	for rawSource, rawReplacement := range root.PawnKit.DependencyOverrides {
		source, err := manifest.ParseDependency(rawSource)
		if err != nil || source.RefKind != manifest.RefNone {
			return nil, fmt.Errorf("dependency: invalid override identity %q", rawSource)
		}
		replacement, err := manifest.ParseDependency(rawReplacement)
		if err != nil {
			return nil, fmt.Errorf("dependency: invalid override replacement %q: %w", rawReplacement, err)
		}
		if source.Scheme != replacement.Scheme {
			return nil, fmt.Errorf("dependency: override %q changes dependency scheme", rawSource)
		}
		key := dependencyKey(source)
		if _, exists := overrides[key]; exists {
			return nil, fmt.Errorf("dependency: duplicate override identity %q", rawSource)
		}
		overrides[key] = replacement
	}
	return overrides, nil
}

func applyDependencyOverride(
	dep manifest.Dependency,
	overrides map[string]manifest.Dependency,
) (manifest.Dependency, error) {
	seen := make(map[string]bool, len(overrides))
	for {
		key := dependencyKey(dep)
		replacement, ok := overrides[key]
		if !ok {
			return dep, nil
		}
		if dependencyKey(replacement) == key {
			return replacement, nil
		}
		if seen[key] {
			return manifest.Dependency{}, fmt.Errorf("dependency: override cycle detected at %s", key)
		}
		seen[key] = true
		dep = replacement
	}
}

func constraintsConflict(
	previous, current string,
	pkg *lockfile.Package,
	request resolutionRequest,
) bool {
	if previous == current {
		return false
	}
	return pkg == nil || pkg.Transitive || request.direct
}

func sortedResolvedPackages(packages map[string]*lockfile.Package) []lockfile.Package {
	result := make([]lockfile.Package, 0, len(packages))
	for _, pkg := range packages {
		sort.Strings(pkg.RequiredBy)
		sort.Strings(pkg.Dependencies)
		result = append(result, *pkg)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result
}

func conflictingConstraintsError(key, previous, constraint string) error {
	return fmt.Errorf(
		"dependency: conflicting constraints for %s: %q and %q",
		key,
		previous,
		constraint,
	)
}

func reusableLockedPackage(
	locked map[string]lockfile.Package,
	key, constraint string,
	update bool,
) *lockfile.Package {
	if update {
		return nil
	}
	candidate, ok := locked[key]
	if !ok || candidate.Constraint != constraint {
		return nil
	}
	return &candidate
}

func rootResolutionRequests(root *manifest.Manifest) []resolutionRequest {
	requests := make([]resolutionRequest, 0, len(root.Dependencies)+len(root.DevDependencies))
	for _, dependency := range root.Dependencies {
		requests = append(requests, resolutionRequest{dependency: dependency, direct: true})
	}
	for _, dependency := range root.DevDependencies {
		requests = append(requests, resolutionRequest{dependency: dependency, direct: true, dev: true})
	}
	return requests
}

func lockedPackages(lock *lockfile.Lock) map[string]lockfile.Package {
	if lock == nil {
		return nil
	}
	packages := make(map[string]lockfile.Package, len(lock.Packages))
	for _, pkg := range lock.Packages {
		packages[pkg.Key] = pkg
	}
	return packages
}

func mergeResolutionEdge(
	packages map[string]*lockfile.Package,
	key string,
	request resolutionRequest,
) {
	pkg := packages[key]
	if request.direct {
		pkg.Transitive = false
		pkg.Kind = dependencyKind(request.dependency, request.dev)
	}
	if request.parent != "" {
		pkg.RequiredBy = appendUnique(pkg.RequiredBy, request.parent)
		addForwardDependency(packages, request.parent, pkg.Name)
	}
}

func addForwardDependency(packages map[string]*lockfile.Package, parent, child string) {
	if pkg := packages[parent]; pkg != nil {
		pkg.Dependencies = appendUnique(pkg.Dependencies, child)
	}
}

func appendUnique(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func dependencyKey(dependency manifest.Dependency) string {
	if dependency.Scheme != manifest.SchemeDependency {
		return string(dependency.Scheme) + "://" + dependency.Name()
	}
	return dependencyCanonicalID(dependency)
}

func dependencyCanonicalID(dependency manifest.Dependency) string {
	site := dependency.Site
	if site == "" {
		site = "github.com"
	}
	return site + "/" + dependency.Name()
}

func dependencyConstraint(dependency manifest.Dependency) string {
	switch dependency.RefKind {
	case manifest.RefTag:
		return ":" + dependency.Ref
	case manifest.RefBranch:
		return "@" + dependency.Ref
	case manifest.RefCommit:
		return "#" + dependency.Ref
	default:
		return ""
	}
}

func dependencyKind(dependency manifest.Dependency, dev bool) string {
	if dependency.Scheme != manifest.SchemeDependency {
		return string(dependency.Scheme)
	}
	if dev {
		return lockfile.KindDevDependency
	}
	return lockfile.KindDependency
}

func dependencyBranch(dependency manifest.Dependency) string {
	if dependency.RefKind == manifest.RefBranch {
		return dependency.Ref
	}
	return ""
}
