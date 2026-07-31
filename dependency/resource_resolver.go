package dependency

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/pathutil"
)

// ReleaseProvider lists assets for a locked package revision.
type ReleaseProvider interface {
	Assets(context.Context, lockfile.Package) ([]ReleaseAsset, error)
}

// ResourceResolver builds complete resource records from restored packages.
type ResourceResolver struct {
	fsys       fsx.FS
	downloader ResourceDownloader
	provider   ReleaseProvider
}

// NewResourceResolver creates a package resource resolver.
func NewResourceResolver(
	fsys fsx.FS,
	downloader ResourceDownloader,
	provider ReleaseProvider,
) *ResourceResolver {
	return &ResourceResolver{
		fsys:       fsys,
		downloader: downloader,
		provider:   provider,
	}
}

// Resolve updates the locked resource set for target.
func (r *ResourceResolver) Resolve(
	ctx context.Context,
	root, target, runtimeVersion string,
	lock *lockfile.Lock,
) ([]lockfile.ResolvedResource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if lock == nil {
		return nil, errors.New("dependency: lockfile is required")
	}
	if r.fsys == nil || r.downloader == nil || r.provider == nil {
		return nil, errors.New("dependency: resource resolver is not configured")
	}

	packages := append([]lockfile.Package(nil), lock.Packages...)
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})
	resolved := make([]lockfile.ResolvedResource, 0, len(packages))
	for _, pkg := range packages {
		if !resourcePackageKind(pkg.Kind) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pkgManifest, err := r.loadResourceManifest(root, pkg)
		if err != nil {
			return nil, fmt.Errorf("dependency: resolving resources for %s: %w", pkg.Name, err)
		}
		selected, err := SelectManifestResource(pkgManifest.Resources, target, runtimeVersion)
		if err != nil {
			return nil, fmt.Errorf("dependency: resolving resources for %s: %w", pkg.Name, err)
		}
		assets, err := r.provider.Assets(ctx, pkg)
		if err != nil {
			return nil, fmt.Errorf("dependency: listing assets for %s: %w", pkg.Name, err)
		}
		asset, err := SelectReleaseAsset(selected.Name, assets)
		if err != nil {
			return nil, fmt.Errorf("dependency: selecting asset for %s: %w", pkg.Name, err)
		}
		record, err := ResolveResourceRecord(
			ctx,
			r.downloader,
			resourcePackageKey(pkg),
			target,
			selected,
			asset,
		)
		if err != nil {
			return nil, fmt.Errorf("dependency: resolving asset for %s: %w", pkg.Name, err)
		}
		resolved = append(resolved, record)
	}

	for _, existing := range lock.Resources {
		if existing.Target != target {
			resolved = append(resolved, existing)
		}
	}
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].Package != resolved[j].Package {
			return resolved[i].Package < resolved[j].Package
		}
		if resolved[i].Resource != resolved[j].Resource {
			return resolved[i].Resource < resolved[j].Resource
		}
		return resolved[i].Target < resolved[j].Target
	})
	return resolved, nil
}

func (r *ResourceResolver) loadResourceManifest(
	root string,
	pkg lockfile.Package,
) (*manifest.Manifest, error) {
	directory := pathutil.Join(root, "dependencies", packageRepo(pkg.Name))
	if pkg.Source.Type == lockfile.SourceTypeLocal {
		var err error
		directory, err = pathutil.SafeJoin(root, pkg.Source.URL)
		if err != nil {
			return nil, err
		}
	}
	manifestPath := pathutil.Join(directory, "pawn.json")
	if _, err := r.fsys.Stat(manifestPath); errors.Is(err, fs.ErrNotExist) {
		manifestPath = pathutil.Join(directory, "pawn.yaml")
	} else if err != nil {
		return nil, fmt.Errorf("checking package manifest: %w", err)
	}
	result, err := manifest.Load(source.NewRegistry(), r.fsys, manifestPath)
	if err != nil {
		return nil, err
	}
	if result.Manifest == nil {
		return nil, errors.New("package manifest could not be decoded")
	}
	if len(result.Diagnostics) != 0 {
		return nil, fmt.Errorf("package manifest: %s", result.Diagnostics[0].Message)
	}
	if len(result.Manifest.Resources) == 0 {
		return nil, errors.New("package manifest does not declare resources")
	}
	return result.Manifest, nil
}

func resourcePackageKind(kind string) bool {
	switch kind {
	case lockfile.KindPlugin,
		lockfile.KindComponent,
		lockfile.KindFilterscript,
		lockfile.KindIncludes:
		return true
	default:
		return false
	}
}

func resourcePackageKey(pkg lockfile.Package) string {
	return pkg.Kind + "://" + pkg.Name
}
