package dependency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxIntegrityFiles     = 100_000
	maxIntegrityFileBytes = 64 * 1024 * 1024
	maxIntegrityTotal     = 1024 * 1024 * 1024
)

var errIntegrityMismatch = errors.New("dependency: integrity mismatch")

func verifyDirectoryIntegrity(root, expected string) error {
	if expected == "" {
		return nil
	}

	files, err := integrityFiles(root)
	if err != nil {
		return err
	}

	hash := sha256.New()
	var total int64
	for _, relative := range files {
		info, err := os.Lstat(filepath.Join(root, relative))
		if err != nil {
			return fmt.Errorf("dependency: reading integrity file %q: %w", relative, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("dependency: integrity file %q is a symbolic link", relative)
		}
		if info.Size() > maxIntegrityFileBytes || total+info.Size() > maxIntegrityTotal {
			return errors.New("dependency: integrity input exceeds size limit")
		}
		total += info.Size()

		_, _ = hash.Write([]byte(relative))
		if err := hashIntegrityFile(hash, filepath.Join(root, relative)); err != nil {
			return err
		}
	}

	actual := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("%w: got %s, want %s", errIntegrityMismatch, actual, expected)
	}
	return nil
}

func integrityFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !integrityExtension(filepath.Ext(path)) {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, relative)
		if len(files) > maxIntegrityFiles {
			return errors.New("dependency: integrity input exceeds file limit")
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("dependency: walking integrity files: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func integrityExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".inc", ".pwn", ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func hashIntegrityFile(hash io.Writer, path string) (err error) {
	file, err := os.Open(path) //nolint:gosec // path is contained by the staged checkout.
	if err != nil {
		return fmt.Errorf("dependency: opening integrity file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if _, err := io.Copy(hash, io.LimitReader(file, maxIntegrityFileBytes+1)); err != nil {
		return fmt.Errorf("dependency: hashing integrity file: %w", err)
	}
	return nil
}
