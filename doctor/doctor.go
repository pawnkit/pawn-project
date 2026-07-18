// Package doctor runs safe project and environment checks.
package doctor

import (
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/paths"
)

// Status is the outcome of one check.
type Status int

const (
	StatusOK Status = iota
	StatusWarning
	StatusFail
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusWarning:
		return "warning"
	case StatusFail:
		return "fail"
	default:
		return "unknown"
	}
}

// Environment is the read-only context inspected by checks.
type Environment struct {
	FS       fsx.FS
	Root     string
	Manifest *manifest.Manifest
	Lock     *lockfile.Lock
	Paths    paths.Resolved
}

// Result is one check's outcome.
type Result struct {
	Name    string
	Status  Status
	Message string
}

// Check inspects a project without executing project code.
type Check interface {
	Name() string
	Run(env Environment) Result
}

// Run executes every check in checks against env, in order, and returns
// one Result per check.
func Run(env Environment, checks []Check) []Result {
	out := make([]Result, len(checks))

	for i, c := range checks {
		out[i] = c.Run(env)
	}

	return out
}

// DefaultChecks returns the built-in project checks.
func DefaultChecks() []Check {
	return []Check{
		ManifestParsesCheck{},
		IncludePathsExistCheck{},
		ManifestDriftCheck{},
	}
}
