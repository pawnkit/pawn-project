package dependency

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
)

// SelectTag returns the newest tag matching a dependency constraint.
func SelectTag(tags []string, constraint string) (string, error) {
	if constraint == "" {
		return "", errors.New("dependency: tag constraint is required")
	}
	for _, tag := range tags {
		if tag == constraint {
			return tag, nil
		}
	}

	rule, err := semver.NewConstraint(constraint)
	if err != nil {
		return "", fmt.Errorf("dependency: invalid tag constraint %q: %w", constraint, err)
	}
	type candidate struct {
		name    string
		version *semver.Version
	}
	matches := make([]candidate, 0, len(tags))
	for _, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err == nil && rule.Check(version) {
			matches = append(matches, candidate{name: tag, version: version})
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("dependency: no tag matches %q", constraint)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].version.Equal(matches[j].version) {
			return matches[i].name < matches[j].name
		}
		return matches[i].version.GreaterThan(matches[j].version)
	})
	return matches[0].name, nil
}
