package toolchain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"github.com/pawnkit/pawn-project/pathutil"
	"github.com/pawnkit/pawnkit-core/hash"
)

const maxIndexBytes = 1 << 20

var checksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Index contains reviewed compiler artifacts.
type Index struct {
	SchemaVersion int                `json:"schemaVersion"`
	ID            string             `json:"id"`
	GeneratedAt   string             `json:"generatedAt"`
	Artifacts     []CompilerArtifact `json:"artifacts"`
}

// CompilerArtifact identifies one compiler archive.
type CompilerArtifact struct {
	Vendor     Vendor             `json:"vendor"`
	Version    string             `json:"version"`
	Profiles   []string           `json:"profiles"`
	Target     string             `json:"target"`
	Source     CompilerSource     `json:"source"`
	Archive    CompilerArchive    `json:"archive"`
	Executable CompilerExecutable `json:"executable"`
}

type CompilerSource struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Commit     string `json:"commit"`
}

type CompilerArchive struct {
	URL      string `json:"url"`
	Format   string `json:"format"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

type CompilerExecutable struct {
	Path         string `json:"path"`
	Architecture string `json:"architecture"`
	Checksum     string `json:"checksum"`
}

// LoadIndex reads a compiler index with an expected outer checksum.
func LoadIndex(reader io.Reader, expectedChecksum string) (Index, error) {
	if reader == nil {
		return Index{}, errors.New("toolchain: compiler index reader is nil")
	}
	if !checksumPattern.MatchString(expectedChecksum) {
		return Index{}, errors.New("toolchain: compiler index requires a sha256 checksum")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxIndexBytes+1))
	if err != nil {
		return Index{}, fmt.Errorf("toolchain: reading compiler index: %w", err)
	}
	if len(raw) > maxIndexBytes {
		return Index{}, fmt.Errorf("toolchain: compiler index exceeds %d bytes", maxIndexBytes)
	}
	if actual := hash.Content(raw); actual != expectedChecksum {
		return Index{}, fmt.Errorf("%w: compiler index got %s, want %s", ErrChecksumMismatch, actual, expectedChecksum)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var index Index
	if err := decoder.Decode(&index); err != nil {
		return Index{}, fmt.Errorf("toolchain: decoding compiler index: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Index{}, errors.New("toolchain: compiler index contains multiple JSON values")
		}
		return Index{}, fmt.Errorf("toolchain: decoding compiler index: %w", err)
	}
	if err := index.validate(); err != nil {
		return Index{}, err
	}
	return index, nil
}

// Select returns the exact compiler artifact for target.
func (index Index) Select(vendor Vendor, version, target string) (CompilerArtifact, error) {
	for _, artifact := range index.Artifacts {
		if artifact.Vendor == vendor && artifact.Version == version && artifact.Target == target {
			return artifact, nil
		}
	}
	return CompilerArtifact{}, fmt.Errorf("%w: %s/%s/%s", ErrNotFound, vendor, version, target)
}

func (index Index) validate() error {
	if index.SchemaVersion != 1 || index.ID == "" || index.GeneratedAt == "" || len(index.Artifacts) == 0 {
		return errors.New("toolchain: compiler index is missing required fields")
	}
	seen := make(map[string]bool, len(index.Artifacts))
	for _, artifact := range index.Artifacts {
		if err := artifact.validate(); err != nil {
			return err
		}
		key := string(artifact.Vendor) + "\x00" + artifact.Version + "\x00" + artifact.Target
		if seen[key] {
			return fmt.Errorf(
				"toolchain: duplicate compiler coordinate %s/%s/%s",
				artifact.Vendor, artifact.Version, artifact.Target,
			)
		}
		seen[key] = true
	}
	return nil
}

func (artifact CompilerArtifact) validate() error {
	if err := validateCoordinate(artifact.Vendor, artifact.Version); err != nil {
		return err
	}
	if artifact.Target == "" || len(artifact.Profiles) == 0 || artifact.Archive.Size < 1 {
		return errors.New("toolchain: compiler artifact is missing required fields")
	}
	switch artifact.Archive.Format {
	case "raw", "zip", "tar.gz":
	default:
		return fmt.Errorf("toolchain: unsupported compiler archive format %q", artifact.Archive.Format)
	}
	parsed, err := url.Parse(artifact.Archive.URL)
	if err != nil || parsed.Scheme != httpsScheme || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("toolchain: invalid compiler archive URL %q", artifact.Archive.URL)
	}
	if !checksumPattern.MatchString(artifact.Archive.Checksum) ||
		!checksumPattern.MatchString(artifact.Executable.Checksum) {
		return errors.New("toolchain: compiler artifact requires archive and executable checksums")
	}
	if pathutil.IsAbs(artifact.Executable.Path) || pathutil.HasTraversal(artifact.Executable.Path) ||
		strings.ContainsRune(artifact.Executable.Path, '\\') {
		return fmt.Errorf("toolchain: unsafe compiler executable path %q", artifact.Executable.Path)
	}
	return nil
}
