package toolchain

import (
	"errors"
	"fmt"
	"os/exec"
)

// PathLookup finds executables using the host's command search rules.
type PathLookup interface {
	LookPath(string) (string, error)
}

// OSPathLookup searches the current process PATH.
type OSPathLookup struct{}

func (OSPathLookup) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// FindCompiler returns the first compiler found in candidate order.
func FindCompiler(lookup PathLookup, candidates ...string) (string, error) {
	if lookup == nil {
		return "", errors.New("toolchain: path lookup is not configured")
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path, err := lookup.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w on PATH", ErrNotFound)
}
