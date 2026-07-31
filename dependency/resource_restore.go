package dependency

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/pathutil"
)

// ResourceResult describes one restored locked resource.
type ResourceResult struct {
	Package  string `json:"package"`
	Resource string `json:"resource"`
	Target   string `json:"target"`
	Files    int    `json:"files"`
}

// ResourceRestorer verifies and installs resources for one host target.
type ResourceRestorer struct {
	fsys       ResourceFS
	downloader ResourceDownloader
}

// NewResourceRestorer creates a locked resource restorer.
func NewResourceRestorer(
	fsys ResourceFS,
	downloader ResourceDownloader,
) *ResourceRestorer {
	return &ResourceRestorer{fsys: fsys, downloader: downloader}
}

// Restore installs every locked resource for target below root.
func (r *ResourceRestorer) Restore(
	ctx context.Context,
	root, target string,
	lock *lockfile.Lock,
) ([]ResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if lock == nil {
		return nil, errors.New("dependency: lockfile is required")
	}
	if r.fsys == nil || r.downloader == nil {
		return nil, errors.New("dependency: resource restorer is not configured")
	}
	if strings.TrimSpace(target) == "" {
		return nil, errors.New("dependency: resource target is required")
	}

	resources, err := selectTargetResources(lock.Resources, target)
	if err != nil {
		return nil, err
	}
	if err := validateResourceDestinations(resources); err != nil {
		return nil, err
	}
	if len(resources) == 0 {
		return nil, nil
	}

	payload := make(ResourcePayload)
	results := make([]ResourceResult, 0, len(resources))
	for _, resource := range resources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		files, err := FetchResource(ctx, r.downloader, resource)
		if err != nil {
			return nil, fmt.Errorf(
				"dependency: restoring resource %s/%s: %w",
				resource.Package,
				resource.Resource,
				err,
			)
		}
		maps.Copy(payload, files)
		results = append(results, ResourceResult{
			Package:  resource.Package,
			Resource: resource.Resource,
			Target:   resource.Target,
			Files:    len(files),
		})
	}

	if err := InstallResourcePayload(ctx, r.fsys, root, payload); err != nil {
		return nil, fmt.Errorf("dependency: installing locked resources: %w", err)
	}
	return results, nil
}

func selectTargetResources(
	resources []lockfile.ResolvedResource,
	target string,
) ([]lockfile.ResolvedResource, error) {
	type resourceSet struct {
		selected *lockfile.ResolvedResource
		targets  []string
	}
	sets := make(map[string]*resourceSet)
	for i := range resources {
		resource := &resources[i]
		key := resource.Package + "\x00" + resource.Resource
		set := sets[key]
		if set == nil {
			set = &resourceSet{}
			sets[key] = set
		}
		set.targets = append(set.targets, resource.Target)
		if resource.Target == target {
			set.selected = resource
		}
	}

	selected := make([]lockfile.ResolvedResource, 0, len(sets))
	for key, set := range sets {
		if set.selected == nil {
			pkg, resource, _ := strings.Cut(key, "\x00")
			sort.Strings(set.targets)
			return nil, fmt.Errorf(
				"dependency: resource %s/%s has no record for target %q; available targets: %s",
				pkg,
				resource,
				target,
				strings.Join(set.targets, ", "),
			)
		}
		selected = append(selected, *set.selected)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Package != selected[j].Package {
			return selected[i].Package < selected[j].Package
		}
		return selected[i].Resource < selected[j].Resource
	})
	return selected, nil
}

func validateResourceDestinations(resources []lockfile.ResolvedResource) error {
	seen := make(map[string]string)
	for _, resource := range resources {
		for _, file := range resource.Files {
			destination := pathutil.Clean(file.Destination)
			key := strings.ToLower(destination)
			owner := resource.Package + "/" + resource.Resource
			if previous, ok := seen[key]; ok {
				return fmt.Errorf(
					"dependency: resource destination %q is shared by %s and %s",
					destination,
					previous,
					owner,
				)
			}
			seen[key] = owner
		}
	}
	return nil
}
