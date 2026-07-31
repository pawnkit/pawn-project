package dependency

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/pawnkit/pawn-project/pathutil"
)

// ResourceFS provides the writes needed for resource installation.
type ResourceFS interface {
	Stat(string) (fs.FileInfo, error)
	Lstat(string) (fs.FileInfo, error)
	MkdirAll(string) error
	MkdirTemp(string, string) (string, error)
	WriteFile(string, []byte) error
	RemoveAll(string) error
	Rename(string, string) error
}

// OSResourceFS installs resources on the host filesystem.
type OSResourceFS struct{}

func (OSResourceFS) Stat(path string) (fs.FileInfo, error)  { return os.Stat(path) }
func (OSResourceFS) Lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }
func (OSResourceFS) MkdirAll(path string) error             { return os.MkdirAll(path, 0o750) }
func (OSResourceFS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func (OSResourceFS) WriteFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0o600)
}
func (OSResourceFS) RemoveAll(path string) error          { return os.RemoveAll(path) }
func (OSResourceFS) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }

type resourceInstallEntry struct {
	relative  string
	target    string
	staged    string
	backup    string
	hadTarget bool
	installed bool
}

// InstallResourcePayload commits a verified payload below root.
func InstallResourcePayload(
	ctx context.Context,
	fsys ResourceFS,
	root string,
	payload ResourcePayload,
) (err error) {
	if fsys == nil {
		return errors.New("dependency: resource filesystem is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	normalized, destinations, err := validatePayloadDestinations(root, payload)
	if err != nil {
		return err
	}

	metadataDir, err := pathutil.SafeJoin(root, ".pawnkit")
	if err != nil {
		return err
	}
	if err := prepareResourceMetadataDir(fsys, metadataDir); err != nil {
		return err
	}
	transactionDir, err := fsys.MkdirTemp(metadataDir, "resource-")
	if err != nil {
		return fmt.Errorf("dependency: creating resource transaction: %w", err)
	}
	defer func() {
		if cleanupErr := fsys.RemoveAll(transactionDir); cleanupErr != nil && err == nil {
			err = fmt.Errorf("dependency: cleaning resource transaction: %w", cleanupErr)
		}
	}()

	stageDir := pathutil.Join(transactionDir, "stage")
	backupDir := pathutil.Join(transactionDir, "backup")
	entries := make([]resourceInstallEntry, 0, len(destinations))
	for _, relative := range destinations {
		if err := ctx.Err(); err != nil {
			return err
		}
		target, _ := pathutil.SafeJoin(root, relative)
		staged, _ := pathutil.SafeJoin(stageDir, relative)
		backup, _ := pathutil.SafeJoin(backupDir, relative)
		if err := fsys.MkdirAll(pathutil.Dir(staged)); err != nil {
			return fmt.Errorf("dependency: creating resource staging directory: %w", err)
		}
		if err := fsys.WriteFile(staged, normalized[relative]); err != nil {
			return fmt.Errorf("dependency: staging resource %q: %w", relative, err)
		}
		entries = append(entries, resourceInstallEntry{
			relative: relative,
			target:   target,
			staged:   staged,
			backup:   backup,
		})
	}

	if err := commitResourceEntries(ctx, fsys, root, entries); err != nil {
		rollbackErr := rollbackResourceEntries(fsys, entries)
		if rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	return nil
}

func validatePayloadDestinations(
	root string,
	payload ResourcePayload,
) (ResourcePayload, []string, error) {
	if len(payload) == 0 {
		return nil, nil, errors.New("dependency: resource payload is empty")
	}
	normalized := make(ResourcePayload, len(payload))
	destinations := make([]string, 0, len(payload))
	seen := make(map[string]string, len(payload))
	for destination := range payload {
		clean := pathutil.Clean(destination)
		if destination == "" || destination == "." || strings.ContainsRune(destination, '\\') ||
			pathutil.IsAbs(destination) || pathutil.HasTraversal(destination) ||
			clean == ".pawnkit" || strings.HasPrefix(clean, ".pawnkit/") {
			return nil, nil, fmt.Errorf("dependency: unsafe resource destination %q", destination)
		}
		if _, err := pathutil.SafeJoin(root, clean); err != nil {
			return nil, nil, fmt.Errorf("dependency: unsafe resource destination %q: %w", destination, err)
		}
		key := strings.ToLower(clean)
		if previous, ok := seen[key]; ok {
			return nil, nil, fmt.Errorf(
				"dependency: resource destinations %q and %q collide",
				previous,
				destination,
			)
		}
		seen[key] = destination
		normalized[clean] = payload[destination]
		destinations = append(destinations, clean)
	}
	sort.Strings(destinations)
	return normalized, destinations, nil
}

func prepareResourceMetadataDir(fsys ResourceFS, metadataDir string) error {
	info, err := fsys.Lstat(metadataDir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := fsys.MkdirAll(metadataDir); err != nil {
			return fmt.Errorf("dependency: creating resource metadata directory: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("dependency: checking resource metadata directory: %w", err)
	case info.Mode()&fs.ModeSymlink != 0:
		return errors.New("dependency: resource metadata directory is a symlink")
	case !info.IsDir():
		return errors.New("dependency: resource metadata path is not a directory")
	default:
		return nil
	}
}

func commitResourceEntries(
	ctx context.Context,
	fsys ResourceFS,
	root string,
	entries []resourceInstallEntry,
) error {
	for i := range entries {
		entry := &entries[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := rejectSymlinkParents(fsys, root, entry.relative); err != nil {
			return err
		}
		info, statErr := fsys.Stat(entry.target)
		switch {
		case statErr == nil && info.IsDir():
			return fmt.Errorf("dependency: resource destination %q is a directory", entry.relative)
		case statErr == nil:
			if err := fsys.MkdirAll(pathutil.Dir(entry.backup)); err != nil {
				return fmt.Errorf("dependency: creating resource backup directory: %w", err)
			}
			if err := fsys.Rename(entry.target, entry.backup); err != nil {
				return fmt.Errorf("dependency: preserving resource %q: %w", entry.relative, err)
			}
			entry.hadTarget = true
		case !errors.Is(statErr, fs.ErrNotExist):
			return fmt.Errorf("dependency: checking resource %q: %w", entry.relative, statErr)
		}
		if err := fsys.MkdirAll(pathutil.Dir(entry.target)); err != nil {
			return fmt.Errorf("dependency: creating destination for %q: %w", entry.relative, err)
		}
		if err := fsys.Rename(entry.staged, entry.target); err != nil {
			return fmt.Errorf("dependency: installing resource %q: %w", entry.relative, err)
		}
		entry.installed = true
	}
	return nil
}

func rollbackResourceEntries(fsys ResourceFS, entries []resourceInstallEntry) error {
	var rollbackErrors []error
	for i := range slices.Backward(entries) {
		entry := entries[i]
		if entry.installed {
			if err := fsys.RemoveAll(entry.target); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("removing %q: %w", entry.relative, err))
				continue
			}
		}
		if entry.hadTarget {
			if err := fsys.Rename(entry.backup, entry.target); err != nil {
				rollbackErrors = append(rollbackErrors, fmt.Errorf("restoring %q: %w", entry.relative, err))
			}
		}
	}
	if len(rollbackErrors) != 0 {
		return fmt.Errorf("dependency: rolling back resources: %w", errors.Join(rollbackErrors...))
	}
	return nil
}

func rejectSymlinkParents(fsys ResourceFS, root, relative string) error {
	current := root
	segments := strings.Split(pathutil.ToSlash(relative), "/")
	for _, segment := range segments[:len(segments)-1] {
		current = pathutil.Join(current, segment)
		info, err := fsys.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("dependency: checking resource parent %q: %w", current, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("dependency: resource parent %q is a symlink", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("dependency: resource parent %q is not a directory", current)
		}
	}
	return nil
}
