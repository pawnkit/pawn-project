package dependency

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/pawnkit/pawn-project/manifest"
)

// ReleaseAsset is one downloadable package release file.
type ReleaseAsset struct {
	Name string
	URL  string
	Size int64
}

// SelectManifestResource applies sampctl's platform and version precedence.
func SelectManifestResource(
	resources []manifest.Resource,
	target, runtimeVersion string,
) (manifest.Resource, error) {
	platform, _, ok := strings.Cut(target, "-")
	if !ok || platform == "" {
		return manifest.Resource{}, fmt.Errorf("dependency: invalid resource target %q", target)
	}

	var fallback *manifest.Resource
	for i := range resources {
		resource := &resources[i]
		if resource.Platform != platform {
			continue
		}
		if runtimeVersion != "" && resource.Version == runtimeVersion {
			return *resource, nil
		}
		if fallback == nil && resource.Version == "" {
			fallback = resource
		}
	}
	if fallback != nil {
		return *fallback, nil
	}
	return manifest.Resource{}, fmt.Errorf(
		"dependency: package has no resource for target %q and runtime %q",
		target,
		runtimeVersion,
	)
}

// SelectReleaseAsset returns the first safe matching asset.
func SelectReleaseAsset(pattern string, assets []ReleaseAsset) (ReleaseAsset, error) {
	matcher, err := regexp.Compile(pattern)
	if err != nil {
		return ReleaseAsset{}, fmt.Errorf("dependency: invalid resource pattern %q: %w", pattern, err)
	}

	var selected ReleaseAsset
	for _, asset := range assets {
		if matcher.MatchString(asset.Name) {
			selected = asset
			break
		}
	}
	if selected.Name == "" {
		return ReleaseAsset{}, fmt.Errorf("dependency: no release asset matches %q", pattern)
	}

	parsed, err := url.Parse(selected.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return ReleaseAsset{}, fmt.Errorf("dependency: resource asset has unsafe URL %q", selected.URL)
	}
	if selected.Size < 1 || selected.Size > maxResourceBytes {
		return ReleaseAsset{}, fmt.Errorf(
			"dependency: resource asset %q has invalid size %d",
			selected.Name,
			selected.Size,
		)
	}
	return selected, nil
}
