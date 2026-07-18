package manifest

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"
	"gopkg.in/yaml.v3"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/pathutil"
)

// LoadResult contains a manifest and any content diagnostics.
type LoadResult struct {
	Manifest    *Manifest
	Diagnostics []diagnostic.Diagnostic
}

var profilePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var knownPawnKitFields = map[string]bool{
	"schemaVersion": true,
	"profile":       true,
	"includePaths":  true,
	"tests":         true,
	"tool":          true,
	"lockfile":      true,
}

// Load reads and validates an absolute JSON or YAML manifest path. Content
// problems become diagnostics; filesystem and format errors are returned.
func Load(reg *source.Registry, fsys fsx.FS, path string) (LoadResult, error) {
	format, err := formatFor(path)
	if err != nil {
		return LoadResult{}, err
	}

	content, err := fsys.ReadFile(path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("manifest: reading %q: %w", path, err)
	}

	fileID := reg.Intern(source.FileURI(path))

	var raw any

	var parseErr error

	switch format {
	case formatJSON:
		parseErr = json.Unmarshal(content, &raw)
	case formatYAML:
		parseErr = yaml.Unmarshal(content, &raw)
	}

	if parseErr != nil {
		return LoadResult{
			Diagnostics: []diagnostic.Diagnostic{
				parseErrorDiagnostic(fileID, content, parseErr),
			},
		}, nil
	}

	rawMap, ok := raw.(map[string]any)
	if !ok {
		return LoadResult{
			Diagnostics: []diagnostic.Diagnostic{
				newDiagnostic(CodeInvalidType, diagnostic.SeverityError,
					"manifest document must be a JSON/YAML object", wholeFileSpan(fileID, content)),
			},
		}, nil
	}

	canonical, err := json.Marshal(rawMap)
	if err != nil {
		return LoadResult{}, fmt.Errorf("manifest: re-encoding %q: %w", path, err)
	}

	var m Manifest
	if err := json.Unmarshal(canonical, &m); err != nil {
		return LoadResult{}, fmt.Errorf("manifest: decoding %q: %w", path, err)
	}

	m.SourcePath = path

	v := &validator{fileID: fileID, content: content, raw: rawMap, m: &m}
	v.run()

	return LoadResult{Manifest: &m, Diagnostics: v.diags}, nil
}

type fileFormat int

const (
	formatJSON fileFormat = iota
	formatYAML
)

func formatFor(path string) (fileFormat, error) {
	switch pathutil.Ext(path) {
	case ".json":
		return formatJSON, nil
	case ".yaml", ".yml":
		return formatYAML, nil
	default:
		return 0, fmt.Errorf("manifest: unsupported extension for %q (want .json, .yaml, or .yml)", path)
	}
}

func wholeFileSpan(file source.FileID, content []byte) source.Span {
	span, err := source.NewSpan(file, 0, source.Offset(len(content)))
	if err != nil {
		// Content length always forms a valid span.
		panic(fmt.Sprintf("manifest: building whole-file span: %v", err))
	}

	return span
}

func newDiagnostic(code string, severity diagnostic.Severity, message string, span source.Span) diagnostic.Diagnostic {
	return diagnostic.New(code, Source, severity, message, span)
}

func parseErrorDiagnostic(fileID source.FileID, content []byte, err error) diagnostic.Diagnostic {
	span := wholeFileSpan(fileID, content)

	if synErr, ok := err.(*json.SyntaxError); ok { //nolint:errorlint // json.SyntaxError is a concrete sentinel type here, not wrapped.
		off := source.Offset(synErr.Offset)
		if s, spanErr := source.NewSpan(fileID, off, off); spanErr == nil {
			span = s
		}
	}

	return newDiagnostic(CodeParseError, diagnostic.SeverityError,
		fmt.Sprintf("manifest failed to parse: %v", err), span)
}
