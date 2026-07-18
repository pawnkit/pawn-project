// Package profile selects a manifest's profile, build, and runtime.
package profile

import (
	"errors"
	"fmt"

	"github.com/pawnkit/pawn-project/manifest"
)

// preset-to-profile default mapping, per RFC 0002 "Field mapping from
// preset to profiles".
const (
	ProfileSAMP037     = "samp-037"
	ProfileOpenMP      = "openmp"
	ProfileRecommended = "recommended"
	ProfileStrict      = "strict"
	ProfileLegacy      = "legacy"
)

// manifest.Manifest.Preset's accepted values, per pawn-project.schema.json.
const (
	PresetSAMP   = "samp"
	PresetOpenMP = "openmp"
)

var presetDefault = map[string]string{
	PresetSAMP:   ProfileSAMP037,
	PresetOpenMP: ProfileOpenMP,
}

// ErrBuildNotFound and ErrRuntimeNotFound are returned by [Select] when
// opts names a build/runtime that does not exist in the manifest.
var (
	ErrBuildNotFound   = errors.New("profile: named build not found in manifest")
	ErrRuntimeNotFound = errors.New("profile: named runtime not found in manifest")
)

// Options carries CLI-level overrides for profile/build/runtime selection.
type Options struct {
	// ProfileOverride, if non-empty, takes precedence over
	// manifest.pawnkit.profile and the preset-based default.
	ProfileOverride string

	// BuildName selects a named build. Empty selects the default or first build.
	BuildName string

	// RuntimeName selects a named runtime using the same fallback as BuildName.
	RuntimeName string
}

// Selection is the resolved profile/build/runtime for one operation.
type Selection struct {
	// ProfileID is empty when no profile can be selected.
	ProfileID string

	Build   *manifest.Build
	Runtime *manifest.Runtime
}

// Select resolves the active profile/build/runtime for m, applying opts as
// overrides.
func Select(m *manifest.Manifest, opts Options) (Selection, error) {
	sel := Selection{ProfileID: resolveProfileID(m, opts)}

	build, err := selectBuild(m, opts.BuildName)
	if err != nil {
		return Selection{}, err
	}

	sel.Build = build

	runtime, err := selectRuntime(m, opts.RuntimeName)
	if err != nil {
		return Selection{}, err
	}

	sel.Runtime = runtime

	return sel, nil
}

func resolveProfileID(m *manifest.Manifest, opts Options) string {
	if opts.ProfileOverride != "" {
		return opts.ProfileOverride
	}

	if m.PawnKit != nil && m.PawnKit.Profile != "" {
		return m.PawnKit.Profile
	}

	if id, ok := presetDefault[m.Preset]; ok {
		return id
	}

	return ""
}

func selectBuild(m *manifest.Manifest, name string) (*manifest.Build, error) {
	if len(m.Builds) > 0 {
		if name == "" {
			return &m.Builds[0], nil
		}

		for i := range m.Builds {
			if m.Builds[i].Name == name {
				return &m.Builds[i], nil
			}
		}

		return nil, fmt.Errorf("%w: %q", ErrBuildNotFound, name)
	}

	return m.Build, nil
}

func selectRuntime(m *manifest.Manifest, name string) (*manifest.Runtime, error) {
	if len(m.Runtimes) > 0 {
		if name == "" {
			return &m.Runtimes[0], nil
		}

		for i := range m.Runtimes {
			if m.Runtimes[i].Name == name {
				return &m.Runtimes[i], nil
			}
		}

		return nil, fmt.Errorf("%w: %q", ErrRuntimeNotFound, name)
	}

	return m.Runtime, nil
}
