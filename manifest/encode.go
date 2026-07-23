package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	// ProfileOpenMP selects the open.mp target profile.
	ProfileOpenMP = "openmp"
	// ProfileSAMP selects the SA-MP target profile.
	ProfileSAMP = "samp"
)

// CreateOptions describes a new PawnKit project manifest.
type CreateOptions struct {
	Entry        string
	Profile      string
	IncludePaths []string
}

// New creates a manifest with sampctl-compatible fields and optional PawnKit
// settings.
func New(options CreateOptions) (*Manifest, error) {
	if options.Entry == "" {
		return nil, errors.New("manifest: entry is required")
	}
	if !profilePattern.MatchString(options.Profile) {
		return nil, fmt.Errorf("manifest: invalid profile %q", options.Profile)
	}

	buildFile := false

	return &Manifest{
		Entry:        options.Entry,
		Preset:       options.Profile,
		Experimental: Experimental{BuildFile: &buildFile},
		PawnKit: &PawnKitExtension{
			SchemaVersion: 1,
			Profile:       options.Profile,
			IncludePaths:  append([]string(nil), options.IncludePaths...),
		},
	}, nil
}

// EncodeJSON returns an indented pawn.json document with a trailing newline.
func EncodeJSON(manifest *Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, errors.New("manifest: cannot encode nil manifest")
	}

	type manifestAlias Manifest
	wire := struct {
		*manifestAlias
		Dependencies    []string `json:"dependencies,omitempty"`
		DevDependencies []string `json:"dev_dependencies,omitempty"`
	}{
		manifestAlias:   (*manifestAlias)(manifest),
		Dependencies:    dependencyStrings(manifest.Dependencies),
		DevDependencies: dependencyStrings(manifest.DevDependencies),
	}

	content, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("manifest: encode JSON: %w", err)
	}

	return append(content, '\n'), nil
}

func dependencyStrings(dependencies []Dependency) []string {
	if len(dependencies) == 0 {
		return nil
	}

	values := make([]string, len(dependencies))
	for i, dependency := range dependencies {
		values[i] = dependency.Raw
	}

	return values
}
