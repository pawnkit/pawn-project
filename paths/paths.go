// Package paths resolves manifest paths within a project root.
package paths

import (
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/pathutil"
)

// Resolved contains normalized absolute project paths.
type Resolved struct {
	// Root is the absolute project root directory (the manifest's
	// directory).
	Root string

	// Entry is the absolute path to manifest.Entry, or "" if unset.
	Entry string

	// Output is the absolute path to manifest.Output, or "" if unset.
	Output string

	// IncludeRoots are de-duplicated and retain declaration order.
	IncludeRoots []string

	// GeneratedFiles lists known generated project files.
	GeneratedFiles []string
}

// Resolve computes Resolved for m rooted at root, which must already be an
// absolute path (typically workspace.Root.Dir).
func Resolve(root string, m *manifest.Manifest) (Resolved, error) {
	root = pathutil.Clean(root)

	r := Resolved{Root: root}

	if m.Entry != "" {
		p, err := pathutil.SafeJoin(root, m.Entry)
		if err != nil {
			return Resolved{}, err
		}

		r.Entry = p
	}

	if m.Output != "" {
		p, err := pathutil.SafeJoin(root, m.Output)
		if err != nil {
			return Resolved{}, err
		}

		r.Output = p
	}

	seen := make(map[string]bool)

	for _, rel := range m.EffectiveIncludePaths() {
		p, err := pathutil.SafeJoin(root, rel)
		if err != nil {
			return Resolved{}, err
		}

		if !seen[p] {
			seen[p] = true

			r.IncludeRoots = append(r.IncludeRoots, p)
		}
	}

	if m.Experimental.BuildFileEnabled() {
		r.GeneratedFiles = append(r.GeneratedFiles, pathutil.Join(root, "sampctl_build_file.inc"))
	}

	return r, nil
}
