package doctor

import (
	"fmt"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawnkit-core/hash"
)

// ManifestParsesCheck reports whether a manifest was parsed.
type ManifestParsesCheck struct{}

func (ManifestParsesCheck) Name() string { return "manifest-parses" }

func (ManifestParsesCheck) Run(env Environment) Result {
	if env.Manifest == nil {
		return Result{Name: "manifest-parses", Status: StatusFail, Message: "manifest failed to parse"}
	}

	return Result{Name: "manifest-parses", Status: StatusOK, Message: "manifest parsed"}
}

// ManifestDriftCheck compares the lockfile with the current manifest.
type ManifestDriftCheck struct{}

func (ManifestDriftCheck) Name() string { return "manifest-lock-drift" }

func (c ManifestDriftCheck) Run(env Environment) Result {
	if env.Manifest == nil || env.Lock == nil || env.Lock.ManifestChecksum == "" {
		return Result{Name: c.Name(), Status: StatusOK, Message: "no manifest checksum to verify"}
	}
	if env.FS == nil || env.Manifest.SourcePath == "" {
		return Result{Name: c.Name(), Status: StatusFail, Message: "manifest source is unavailable"}
	}
	content, err := env.FS.ReadFile(env.Manifest.SourcePath)
	if err != nil {
		return Result{Name: c.Name(), Status: StatusFail, Message: fmt.Sprintf("cannot read manifest: %v", err)}
	}
	if actual := hash.Content(content); actual != env.Lock.ManifestChecksum {
		return Result{Name: c.Name(), Status: StatusWarning, Message: "manifest changed since pawn.lock was generated"}
	}
	return Result{Name: c.Name(), Status: StatusOK, Message: "manifest matches pawn.lock"}
}

// IncludePathsExistCheck checks each configured include directory.
type IncludePathsExistCheck struct{}

func (IncludePathsExistCheck) Name() string { return "include-paths-exist" }

func (c IncludePathsExistCheck) Run(env Environment) Result {
	if env.FS == nil {
		return Result{Name: c.Name(), Status: StatusFail, Message: "no filesystem configured"}
	}

	var missing []string

	for _, root := range env.Paths.IncludeRoots {
		if !fsx.IsDir(env.FS, root) {
			missing = append(missing, root)
		}
	}

	switch {
	case len(env.Paths.IncludeRoots) == 0:
		return Result{Name: c.Name(), Status: StatusOK, Message: "no include paths declared"}
	case len(missing) == 0:
		return Result{Name: c.Name(), Status: StatusOK, Message: fmt.Sprintf("%d include path(s) exist", len(env.Paths.IncludeRoots))}
	default:
		return Result{Name: c.Name(), Status: StatusWarning, Message: fmt.Sprintf("missing include paths: %v", missing)}
	}
}
