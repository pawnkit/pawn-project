package dependency

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
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

// SelectReleaseAsset finds one safe asset matching the resource pattern.
func SelectReleaseAsset(pattern string, assets []ReleaseAsset) (ReleaseAsset, error) {
	matcher, err := regexp.Compile(pattern)
	if err != nil {
		return ReleaseAsset{}, fmt.Errorf("dependency: invalid resource pattern %q: %w", pattern, err)
	}
	assets = append([]ReleaseAsset(nil), assets...)
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Name < assets[j].Name
	})

	var matches []ReleaseAsset
	for _, asset := range assets {
		if matcher.MatchString(asset.Name) {
			matches = append(matches, asset)
		}
	}
	if len(matches) == 0 {
		return ReleaseAsset{}, fmt.Errorf("dependency: no release asset matches %q", pattern)
	}
	if len(matches) != 1 {
		names := make([]string, len(matches))
		for i, match := range matches {
			names[i] = match.Name
		}
		return ReleaseAsset{}, fmt.Errorf(
			"dependency: resource pattern %q matches multiple assets: %s",
			pattern,
			strings.Join(names, ", "),
		)
	}

	asset := matches[0]
	parsed, err := url.Parse(asset.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return ReleaseAsset{}, fmt.Errorf("dependency: resource asset has unsafe URL %q", asset.URL)
	}
	if asset.Size < 1 || asset.Size > maxResourceBytes {
		return ReleaseAsset{}, fmt.Errorf(
			"dependency: resource asset %q has invalid size %d",
			asset.Name,
			asset.Size,
		)
	}
	return asset, nil
}
