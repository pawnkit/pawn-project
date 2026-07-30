// Package dependency restores projects from a validated pawn.lock.
package dependency

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/pathutil"
)

// Status describes a restored dependency.
type Status string

const (
	StatusInstalled Status = "installed"
	StatusPresent   Status = "present"
	StatusLocal     Status = "local"
)

// Result describes one restored dependency.
type Result struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Status Status `json:"status"`
}

// Installer installs a locked source at target.
type Installer interface {
	Install(context.Context, lockfile.Package, string) (Status, error)
}

// Restorer installs the dependencies described by a lockfile.
type Restorer struct {
	fsys      fsx.FS
	installer Installer
}

// NewRestorer creates a dependency restorer.
func NewRestorer(fsys fsx.FS, installer Installer) *Restorer {
	return &Restorer{fsys: fsys, installer: installer}
}

// Restore installs missing dependencies below root.
func (r *Restorer) Restore(ctx context.Context, root string, lock *lockfile.Lock) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if lock == nil {
		return nil, errors.New("dependency: lockfile is required")
	}
	if r.fsys == nil || r.installer == nil {
		return nil, errors.New("dependency: restorer is not configured")
	}

	packages, err := orderedPackages(lock)
	if err != nil {
		return nil, err
	}
	if err := validateTargets(root, packages); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(packages))
	for _, pkg := range packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		result, err := r.restorePackage(ctx, root, pkg)
		if err != nil {
			return nil, fmt.Errorf("dependency: restoring %s: %w", pkg.Name, err)
		}

		results = append(results, result)
	}

	return results, nil
}

func validateTargets(root string, packages []lockfile.Package) error {
	targets := make(map[string]string, len(packages))
	for _, pkg := range packages {
		if pkg.Source.Type == lockfile.SourceTypeLocal {
			continue
		}
		repo := packageRepo(pkg.Name)
		if repo == "" {
			return fmt.Errorf("dependency: invalid package name %q", pkg.Name)
		}
		target, err := pathutil.SafeJoin(root, pathutil.Join("dependencies", repo))
		if err != nil {
			return err
		}
		if previous, exists := targets[target]; exists && previous != pkg.Name {
			return fmt.Errorf("dependency: %s and %s share install path %q", previous, pkg.Name, target)
		}
		targets[target] = pkg.Name
	}
	return nil
}

func (r *Restorer) restorePackage(
	ctx context.Context,
	root string,
	pkg lockfile.Package,
) (Result, error) {
	if pkg.Source.Type == lockfile.SourceTypeLocal {
		target, err := pathutil.SafeJoin(root, pkg.Source.URL)
		if err != nil {
			return Result{}, err
		}
		if _, err := r.fsys.Stat(target); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return Result{}, fmt.Errorf("local path %q does not exist", pkg.Source.URL)
			}
			return Result{}, fmt.Errorf("checking local path %q: %w", pkg.Source.URL, err)
		}
		return Result{Name: pkg.Name, Path: target, Status: StatusLocal}, nil
	}

	switch pkg.Kind {
	case lockfile.KindDependency,
		lockfile.KindDevDependency,
		lockfile.KindIncludes,
		lockfile.KindPlugin,
		lockfile.KindComponent,
		lockfile.KindFilterscript:
	default:
		return Result{}, fmt.Errorf("dependency: package kind %q is not supported", pkg.Kind)
	}

	repo := packageRepo(pkg.Name)
	if repo == "" {
		return Result{}, fmt.Errorf("invalid package name %q", pkg.Name)
	}
	target, err := pathutil.SafeJoin(root, pathutil.Join("dependencies", repo))
	if err != nil {
		return Result{}, err
	}

	status, err := r.installer.Install(ctx, pkg, target)
	if err != nil {
		return Result{}, err
	}
	return Result{Name: pkg.Name, Path: target, Status: status}, nil
}

func packageRepo(name string) string {
	if strings.Count(name, "/") != 1 {
		return ""
	}
	_, repo, ok := strings.Cut(name, "/")
	if !ok || repo == "" || repo == "." || repo == ".." || pathutil.IsAbs(repo) || pathutil.HasTraversal(repo) {
		return ""
	}
	return repo
}

func orderedPackages(lock *lockfile.Lock) ([]lockfile.Package, error) {
	byName := make(map[string]lockfile.Package, len(lock.Packages))
	for _, pkg := range lock.Packages {
		byName[pkg.Name] = pkg
	}

	var (
		order    []lockfile.Package
		visiting = make(map[string]bool, len(byName))
		visited  = make(map[string]bool, len(byName))
	)
	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("dependency: cycle contains %s", name)
		}
		pkg, ok := byName[name]
		if !ok {
			return fmt.Errorf("dependency: unknown package %s", name)
		}

		visiting[name] = true
		dependencies := append([]string(nil), pkg.Dependencies...)
		sort.Strings(dependencies)
		for _, dependency := range dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		delete(visiting, name)
		visited[name] = true
		order = append(order, pkg)
		return nil
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return order, nil
}
