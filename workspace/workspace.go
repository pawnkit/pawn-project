// Package workspace finds Pawn project roots.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/pathutil"
)

// ManifestJSON and ManifestYAML are supported manifest names.
const (
	ManifestJSON = "pawn.json"
	ManifestYAML = "pawn.yaml"
)

// manifestNames is the search order applied at each directory level.
var manifestNames = []string{ManifestJSON, ManifestYAML}

// ErrNotFound is returned by [FindRoot] when no manifest is found between
// start and the filesystem root.
var ErrNotFound = errors.New("workspace: no pawn.json or pawn.yaml found")

// Root identifies a discovered project root.
type Root struct {
	Dir          string // Dir is the absolute, slash-separated directory containing the manifest.
	ManifestPath string // ManifestPath is the absolute, slash-separated path to the manifest file that was found.
	ManifestName string // ManifestName is the base filename of the manifest that was found
}

// FindRoot walks upward from an absolute file or directory path.
func FindRoot(fsys fsx.FS, start string) (Root, error) {
	if !pathutil.IsAbs(start) {
		return Root{}, fmt.Errorf("workspace: start path %q must be absolute", start)
	}

	dir := pathutil.Clean(start)

	if info, err := fsys.Stat(dir); err == nil && !info.IsDir() {
		dir = pathutil.Dir(dir)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Root{}, fmt.Errorf("workspace: stat %q: %w", start, err)
	}

	for {
		for _, name := range manifestNames {
			candidate := pathutil.Join(dir, name)
			if fsx.IsFile(fsys, candidate) {
				return Root{Dir: dir, ManifestPath: candidate, ManifestName: name}, nil
			}
		}

		parent := pathutil.Dir(dir)
		if parent == dir {
			return Root{}, fmt.Errorf("%w (searched up from %q)", ErrNotFound, start)
		}

		dir = parent
	}
}
