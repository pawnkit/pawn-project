package toolchain

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"

	"github.com/pawnkit/pawn-project/pathutil"
)

const (
	// These limits bound archive extraction.
	maxArchiveFiles = 10_000
	maxArchiveBytes = 512 * 1024 * 1024
)

// ErrArchiveTraversal is returned when a zip entry's name would extract
// outside the destination directory.
var ErrArchiveTraversal = errors.New("toolchain: archive entry escapes extraction root")

// ErrArchiveTooLarge is returned when an archive exceeds the file-count or
// total-byte extraction limits.
var ErrArchiveTooLarge = errors.New("toolchain: archive exceeds extraction limits")

func isZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04
}

// extractZip safely extracts an archive and returns its compiler path.
func extractZip(fsys CacheFS, dir string, r io.ReaderAt, size int64) (string, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("toolchain: opening archive: %w", err)
	}

	if len(zr.File) > maxArchiveFiles {
		return "", fmt.Errorf("%w: %d entries exceeds limit of %d", ErrArchiveTooLarge, len(zr.File), maxArchiveFiles)
	}

	var (
		totalBytes  int64
		extracted   []string
		binaryGuess string
	)

	for _, f := range zr.File {
		name := pathutil.ToSlash(f.Name)

		if pathutil.IsAbs(name) || pathutil.HasTraversal(name) {
			return "", fmt.Errorf("%w: %q", ErrArchiveTraversal, f.Name)
		}

		if f.FileInfo().IsDir() {
			continue
		}

		destPath, err := pathutil.SafeJoin(dir, name)
		if err != nil {
			return "", fmt.Errorf("%w: %q: %v", ErrArchiveTraversal, f.Name, err) //nolint:errorlint // wrapped intentionally with %v to keep ErrArchiveTraversal as the sentinel.
		}

		if f.UncompressedSize64 > maxArchiveBytes {
			return "", fmt.Errorf("%w: extracted size exceeds %d bytes", ErrArchiveTooLarge, maxArchiveBytes)
		}

		totalBytes += int64(f.UncompressedSize64)
		if totalBytes > maxArchiveBytes {
			return "", fmt.Errorf("%w: extracted size exceeds %d bytes", ErrArchiveTooLarge, maxArchiveBytes)
		}

		content, err := readZipEntry(f)
		if err != nil {
			return "", err
		}

		if err := fsys.MkdirAll(pathutil.Dir(destPath)); err != nil {
			return "", fmt.Errorf("toolchain: creating %q: %w", pathutil.Dir(destPath), err)
		}

		if err := fsys.WriteFile(destPath, content); err != nil {
			return "", fmt.Errorf("toolchain: writing %q: %w", destPath, err)
		}

		base := pathutil.Base(destPath)
		extracted = append(extracted, name)

		if binaryGuess == "" && looksLikeCompilerBinary(base) {
			binaryGuess = name
		}
	}

	if len(extracted) == 0 {
		return "", errors.New("toolchain: archive contained no files")
	}

	if binaryGuess != "" {
		return binaryGuess, nil
	}

	return extracted[0], nil
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("toolchain: opening archive entry %q: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	limited := io.LimitReader(rc, maxArchiveBytes+1)

	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("toolchain: reading archive entry %q: %w", f.Name, err)
	}

	if len(data) > maxArchiveBytes {
		return nil, fmt.Errorf("%w: entry %q exceeds %d bytes", ErrArchiveTooLarge, f.Name, maxArchiveBytes)
	}

	return data, nil
}

func looksLikeCompilerBinary(name string) bool {
	return name == "pawncc" || name == "pawncc.exe"
}
