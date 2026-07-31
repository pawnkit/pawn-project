package dependency

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

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
	Commit    string
	Resolved  string
	SourceURL string
	Manifest  manifest.Manifest
}

// RevisionProvider resolves a manifest dependency to an exact revision.
type RevisionProvider interface {
	Resolve(context.Context, manifest.Dependency, *lockfile.Package) (Revision, error)
}

// GraphResolver builds one deterministic dependency graph.
type GraphResolver struct {
	provider RevisionProvider
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
	queue := rootResolutionRequests(root)
	packages := make(map[string]*lockfile.Package)
	constraints := make(map[string]string)

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request := queue[0]
		queue = queue[1:]
		key := dependencyKey(request.dependency)
		constraint := dependencyConstraint(request.dependency)

		if len(request.ancestry) > maxResolutionDepth {
			return nil, fmt.Errorf("dependency: graph exceeds %d levels at %s", maxResolutionDepth, key)
		}
		if cycleStart := indexOf(request.ancestry, key); cycleStart >= 0 {
			cycle := append(append([]string(nil), request.ancestry[cycleStart:]...), key)
			return nil, fmt.Errorf("dependency: cycle detected: %s", strings.Join(cycle, " -> "))
		}
		if previous, ok := constraints[key]; ok {
			if previous != constraint {
				return nil, fmt.Errorf(
					"dependency: conflicting constraints for %s: %q and %q",
					key,
					previous,
					constraint,
				)
			}
			mergeResolutionEdge(packages, key, request)
			continue
		}
		if len(packages) >= maxResolutionPackages {
			return nil, fmt.Errorf("dependency: graph exceeds %d packages", maxResolutionPackages)
		}

		var reusable *lockfile.Package
		if candidate, ok := locked[key]; ok && candidate.Constraint == constraint {
			candidateCopy := candidate
			reusable = &candidateCopy
		}
		revision, err := r.provider.Resolve(ctx, request.dependency, reusable)
		if err != nil {
			return nil, fmt.Errorf("dependency: resolving %s: %w", key, err)
		}
		if !fullCommitPattern.MatchString(revision.Commit) {
			return nil, fmt.Errorf("dependency: %s resolved invalid commit %q", key, revision.Commit)
		}
		if revision.SourceURL == "" {
			revision.SourceURL = "https://github.com/" + request.dependency.Name()
		}

		pkg := &lockfile.Package{
			Key: key, Constraint: constraint, Name: request.dependency.Name(),
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

	result := make([]lockfile.Package, 0, len(packages))
	for _, pkg := range packages {
		sort.Strings(pkg.RequiredBy)
		sort.Strings(pkg.Dependencies)
		result = append(result, *pkg)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
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
	return "github.com/" + dependency.Name()
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

func indexOf(values []string, value string) int {
	for i, current := range values {
		if current == value {
			return i
		}
	}
	return -1
}
