package dependency

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/pawnkit/pawnkit-core/hash"

	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/pathutil"
)

type resourceArchiveFile struct {
	name    string
	content []byte
}

// ResolveResourceRecord downloads an asset and builds its locked extraction map.
func ResolveResourceRecord(
	ctx context.Context,
	downloader ResourceDownloader,
	packageKey, target string,
	resource manifest.Resource,
	asset ReleaseAsset,
) (lockfile.ResolvedResource, error) {
	if downloader == nil {
		return lockfile.ResolvedResource{}, errors.New("dependency: resource downloader is required")
	}
	if _, err := SelectReleaseAsset(resource.Name, []ReleaseAsset{asset}); err != nil {
		return lockfile.ResolvedResource{}, err
	}
	data, err := downloadResourceAsset(ctx, downloader, asset)
	if err != nil {
		return lockfile.ResolvedResource{}, err
	}

	archive := resourceFormatFile
	files := []resourceArchiveFile{{name: asset.Name, content: data}}
	if resource.Archive {
		archive, files, err = inspectResourceArchive(asset.Name, data)
		if err != nil {
			return lockfile.ResolvedResource{}, err
		}
	}

	resolvedFiles, err := mapResourceFiles(packageKey, resource, files)
	if err != nil {
		return lockfile.ResolvedResource{}, err
	}
	return lockfile.ResolvedResource{
		Package:  packageKey,
		Resource: resource.Name,
		Target:   target,
		URL:      asset.URL,
		Size:     int64(len(data)),
		Checksum: hash.Content(data),
		Archive:  archive,
		Files:    resolvedFiles,
	}, nil
}

func downloadResourceAsset(
	ctx context.Context,
	downloader ResourceDownloader,
	asset ReleaseAsset,
) ([]byte, error) {
	reader, err := downloader.Download(ctx, asset.URL)
	if err != nil {
		return nil, fmt.Errorf("dependency: downloading resource asset: %w", err)
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(io.LimitReader(reader, maxResourceDownload+1))
	if err != nil {
		return nil, fmt.Errorf("dependency: reading resource asset: %w", err)
	}
	if int64(len(data)) != asset.Size {
		return nil, fmt.Errorf(
			"dependency: resource asset size mismatch: got %d, want %d",
			len(data),
			asset.Size,
		)
	}
	return data, nil
}

func inspectResourceArchive(name string, data []byte) (string, []resourceArchiveFile, error) {
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lowerName, ".zip") || bytes.HasPrefix(data, []byte("PK")):
		files, err := inspectZIPResource(data)
		return resourceFormatZIP, files, err
	case strings.HasSuffix(lowerName, ".tar.gz"),
		strings.HasSuffix(lowerName, ".tgz"),
		bytes.HasPrefix(data, []byte{0x1f, 0x8b}):
		files, err := inspectTarResource(data)
		return resourceFormatTar, files, err
	default:
		return "", nil, fmt.Errorf("dependency: resource archive %q has an unsupported format", name)
	}
}

func inspectZIPResource(data []byte) ([]resourceArchiveFile, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("dependency: opening ZIP resource: %w", err)
	}
	if len(reader.File) > maxResourceFiles {
		return nil, fmt.Errorf("dependency: ZIP resource exceeds %d entries", maxResourceFiles)
	}
	files := make([]resourceArchiveFile, 0, len(reader.File))
	seen := make(map[string]string)
	var total uint64
	for _, entry := range reader.File {
		name := pathutil.ToSlash(entry.Name)
		if unsafeArchivePath(name) {
			return nil, fmt.Errorf("dependency: unsafe ZIP entry %q", entry.Name)
		}
		key := strings.ToLower(name)
		if previous, ok := seen[key]; ok {
			return nil, fmt.Errorf("dependency: ZIP entries %q and %q collide", previous, entry.Name)
		}
		seen[key] = entry.Name
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() {
			return nil, fmt.Errorf("dependency: unsupported ZIP entry %q", entry.Name)
		}
		total += entry.UncompressedSize64
		if total > maxResourceBytes {
			return nil, errors.New("dependency: ZIP resource expands beyond 1 GiB")
		}
		if entry.UncompressedSize64 > maxResourceBytes {
			return nil, errors.New("dependency: ZIP entry exceeds 1 GiB")
		}
		content, err := readBounded(entry, int64(entry.UncompressedSize64))
		if err != nil {
			return nil, err
		}
		files = append(files, resourceArchiveFile{name: name, content: content})
	}
	return files, nil
}

func inspectTarResource(data []byte) ([]resourceArchiveFile, error) {
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("dependency: opening tar.gz resource: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	var files []resourceArchiveFile
	seen := make(map[string]string)
	reader := tar.NewReader(gzipReader)
	var total int64
	for count := 0; ; count++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("dependency: reading tar.gz resource: %w", err)
		}
		if count >= maxResourceFiles {
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
		if header.Size < 0 || header.Size > maxResourceBytes-total {
			return nil, errors.New("dependency: tar.gz resource expands beyond 1 GiB")
		}
		total += header.Size
		content, err := readExact(reader, header.Size, header.Size)
		if err != nil {
			return nil, fmt.Errorf("dependency: reading tar entry %q: %w", header.Name, err)
		}
		files = append(files, resourceArchiveFile{name: name, content: content})
	}
	return files, nil
}

func mapResourceFiles(
	packageKey string,
	resource manifest.Resource,
	files []resourceArchiveFile,
) ([]lockfile.ResolvedResourceFile, error) {
	var resolved []lockfile.ResolvedResourceFile
	destinations := make(map[string]string)
	for _, file := range files {
		destination, selected, err := resourceFileDestination(packageKey, resource, file.name)
		if err != nil {
			return nil, err
		}
		if !selected {
			continue
		}
		key := strings.ToLower(destination)
		if previous, ok := destinations[key]; ok {
			return nil, fmt.Errorf(
				"dependency: resource files %q and %q share destination %q",
				previous,
				file.name,
				destination,
			)
		}
		destinations[key] = file.name
		resolved = append(resolved, lockfile.ResolvedResourceFile{
			Source:      file.name,
			Destination: destination,
			Size:        int64(len(file.content)),
			Checksum:    hash.Content(file.content),
		})
	}
	if len(resolved) == 0 {
		return nil, errors.New("dependency: resource does not select any files")
	}
	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].Destination < resolved[j].Destination
	})
	return resolved, nil
}

func resourceFileDestination(
	packageKey string,
	resource manifest.Resource,
	source string,
) (string, bool, error) {
	pluginDir := resourceBinaryDirectory(packageKey)
	if matchResourcePath(source, resource.Plugins) {
		return path.Join(pluginDir, path.Base(source)), true, nil
	}
	if matchResourcePath(source, resource.Includes) {
		return path.Join(resourceIncludeDirectory(packageKey, resource.Name), path.Base(source)), true, nil
	}

	patterns := make([]string, 0, len(resource.Files))
	for pattern := range resource.Files {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	for _, pattern := range patterns {
		matches, err := regexp.MatchString(pattern, source)
		if err != nil {
			matches = pattern == source
		}
		if !matches {
			continue
		}
		destination := pathutil.ToSlash(resource.Files[pattern])
		switch {
		case destination == "":
			destination = path.Base(source)
		case strings.HasSuffix(destination, "/"):
			destination = path.Join(destination, path.Base(source))
		}
		if unsafeArchivePath(destination) {
			return "", false, fmt.Errorf("dependency: unsafe resource destination %q", destination)
		}
		return destination, true, nil
	}

	if !resource.Archive {
		switch {
		case strings.HasPrefix(packageKey, "includes://"):
			return path.Join(resourceIncludeDirectory(packageKey, resource.Name), path.Base(source)), true, nil
		default:
			return path.Join(pluginDir, path.Base(source)), true, nil
		}
	}
	return "", false, nil
}

func matchResourcePath(source string, patterns []string) bool {
	for _, pattern := range patterns {
		matches, err := regexp.MatchString(pattern, source)
		if (err == nil && matches) || (err != nil && pattern == source) {
			return true
		}
	}
	return false
}

func resourceBinaryDirectory(packageKey string) string {
	switch {
	case strings.HasPrefix(packageKey, "component://"):
		return "components"
	case strings.HasPrefix(packageKey, "filterscript://"):
		return "filterscripts"
	default:
		return "plugins"
	}
}

func resourceIncludeDirectory(packageKey, resourceName string) string {
	name := packageKey
	for _, prefix := range []string{
		"plugin://",
		"component://",
		"filterscript://",
		"includes://",
	} {
		name = strings.TrimPrefix(name, prefix)
	}
	repo := path.Base(name)
	if repo == "." || repo == "/" || repo == "" {
		repo = "resource"
	}
	sum := sha256.Sum256([]byte(resourceName))
	suffix := hex.EncodeToString(sum[:3])
	return path.Join("dependencies", ".resources", repo+"-"+suffix)
}
