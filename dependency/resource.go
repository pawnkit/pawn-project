package dependency

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pawnkit/pawnkit-core/hash"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/pathutil"
)

const (
	maxResourceDownload = 1 << 30
	maxResourceFiles    = 4096
	maxResourceBytes    = 1 << 30
	resourceFormatFile  = "file"
	resourceFormatZIP   = "zip"
	resourceFormatTar   = "tar.gz"
)

// ResourceDownloader fetches a locked resource.
type ResourceDownloader interface {
	Download(context.Context, string) (io.ReadCloser, error)
}

// ResourcePayload contains verified destination paths and file data.
type ResourcePayload map[string][]byte

// FetchResource downloads and verifies one resolved resource.
func FetchResource(
	ctx context.Context,
	downloader ResourceDownloader,
	resource lockfile.ResolvedResource,
) (ResourcePayload, error) {
	if downloader == nil {
		return nil, errors.New("dependency: resource downloader is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	reader, err := downloader.Download(ctx, resource.URL)
	if err != nil {
		return nil, fmt.Errorf("dependency: downloading resource: %w", err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(io.LimitReader(reader, maxResourceDownload+1))
	if err != nil {
		return nil, fmt.Errorf("dependency: reading resource: %w", err)
	}
	if int64(len(data)) > maxResourceDownload {
		return nil, errors.New("dependency: resource download exceeds 1 GiB")
	}
	if int64(len(data)) != resource.Size {
		return nil, fmt.Errorf(
			"dependency: resource size mismatch: got %d, want %d",
			len(data),
			resource.Size,
		)
	}
	if actual := hash.Content(data); actual != resource.Checksum {
		return nil, fmt.Errorf(
			"dependency: resource checksum mismatch: got %s, want %s",
			actual,
			resource.Checksum,
		)
	}

	switch resource.Archive {
	case resourceFormatFile:
		return readRawResource(data, resource)
	case resourceFormatZIP:
		return readZipResource(data, resource)
	case resourceFormatTar:
		return readTarResource(data, resource)
	default:
		return nil, fmt.Errorf("dependency: unsupported resource archive %q", resource.Archive)
	}
}

func readRawResource(data []byte, resource lockfile.ResolvedResource) (ResourcePayload, error) {
	if len(resource.Files) != 1 {
		return nil, errors.New("dependency: file resource requires one output")
	}
	file := resource.Files[0]
	if err := verifyResourceFile(file, data); err != nil {
		return nil, err
	}
	return ResourcePayload{file.Destination: data}, nil
}

func readZipResource(data []byte, resource lockfile.ResolvedResource) (ResourcePayload, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("dependency: opening ZIP resource: %w", err)
	}
	if len(reader.File) > maxResourceFiles {
		return nil, fmt.Errorf("dependency: ZIP resource exceeds %d entries", maxResourceFiles)
	}

	entries := make(map[string]*zip.File, len(reader.File))
	var totalBytes uint64
	for _, entry := range reader.File {
		name := pathutil.ToSlash(entry.Name)
		if unsafeArchivePath(name) {
			return nil, fmt.Errorf("dependency: unsafe ZIP entry %q", entry.Name)
		}
		if previous, ok := entries[strings.ToLower(name)]; ok {
			return nil, fmt.Errorf("dependency: ZIP entries %q and %q collide", previous.Name, entry.Name)
		}
		entries[strings.ToLower(name)] = entry
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() {
			return nil, fmt.Errorf("dependency: unsupported ZIP entry %q", entry.Name)
		}
		totalBytes += entry.UncompressedSize64
		if totalBytes > maxResourceBytes {
			return nil, errors.New("dependency: ZIP resource expands beyond 1 GiB")
		}
	}

	payload := make(ResourcePayload, len(resource.Files))
	for _, file := range resource.Files {
		entry, ok := entries[strings.ToLower(file.Source)]
		if !ok || entry.FileInfo().IsDir() {
			return nil, fmt.Errorf("dependency: ZIP resource is missing %q", file.Source)
		}
		content, err := readBounded(entry, file.Size)
		if err != nil {
			return nil, err
		}
		if err := verifyResourceFile(file, content); err != nil {
			return nil, err
		}
		payload[file.Destination] = content
	}
	return payload, nil
}

func readTarResource(data []byte, resource lockfile.ResolvedResource) (ResourcePayload, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("dependency: opening tar.gz resource: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	wanted := make(map[string]lockfile.ResolvedResourceFile, len(resource.Files))
	for _, file := range resource.Files {
		wanted[strings.ToLower(file.Source)] = file
	}
	payload := make(ResourcePayload, len(resource.Files))
	seen := make(map[string]string)
	tarReader := tar.NewReader(gzipReader)
	count := 0
	var totalBytes int64
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("dependency: reading tar.gz resource: %w", err)
		}
		count++
		if count > maxResourceFiles {
			return nil, fmt.Errorf("dependency: tar.gz resource exceeds %d entries", maxResourceFiles)
		}
		name := pathutil.ToSlash(header.Name)
		if unsafeArchivePath(name) {
			return nil, fmt.Errorf("dependency: unsafe tar entry %q", header.Name)
		}
		key := strings.ToLower(name)
		if previous, ok := seen[key]; ok {
			return nil, fmt.Errorf("dependency: tar entries %q and %q collide", previous, header.Name)
		}
		seen[key] = header.Name
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("dependency: unsupported tar entry %q", header.Name)
		}
		if header.Size < 0 || header.Size > maxResourceBytes-totalBytes {
			return nil, errors.New("dependency: tar.gz resource expands beyond 1 GiB")
		}
		totalBytes += header.Size
		file, wantedEntry := wanted[key]
		if !wantedEntry {
			continue
		}
		content, err := readExact(tarReader, header.Size, file.Size)
		if err != nil {
			return nil, fmt.Errorf("dependency: reading tar entry %q: %w", header.Name, err)
		}
		if err := verifyResourceFile(file, content); err != nil {
			return nil, err
		}
		payload[file.Destination] = content
	}
	if len(payload) != len(resource.Files) {
		return nil, errors.New("dependency: tar.gz resource is missing a declared file")
	}
	return payload, nil
}

func readBounded(entry *zip.File, expected int64) ([]byte, error) {
	if expected < 0 || entry.UncompressedSize64 != uint64(expected) {
		return nil, fmt.Errorf("dependency: ZIP entry %q size does not match lock", entry.Name)
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("dependency: opening ZIP entry %q: %w", entry.Name, err)
	}
	defer func() { _ = reader.Close() }()
	return readExact(reader, expected, expected)
}

func readExact(reader io.Reader, size, expected int64) ([]byte, error) {
	if size < 0 || size != expected || size > maxResourceBytes {
		return nil, errors.New("resource size does not match lock")
	}
	data, err := io.ReadAll(io.LimitReader(reader, size+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != size {
		return nil, errors.New("resource file has unexpected size")
	}
	return data, nil
}

func verifyResourceFile(file lockfile.ResolvedResourceFile, data []byte) error {
	if int64(len(data)) != file.Size {
		return fmt.Errorf("dependency: resource file %q size does not match lock", file.Source)
	}
	if actual := hash.Content(data); actual != file.Checksum {
		return fmt.Errorf(
			"dependency: resource file %q checksum mismatch: got %s, want %s",
			file.Source,
			actual,
			file.Checksum,
		)
	}
	return nil
}

func unsafeArchivePath(path string) bool {
	return path == "" || strings.ContainsRune(path, '\\') ||
		pathutil.IsAbs(path) || pathutil.HasTraversal(path)
}
