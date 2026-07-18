package lockfile

import (
	"encoding/json"
	"fmt"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
)

// LoadResult contains a lockfile and any content diagnostics.
type LoadResult struct {
	Lock        *Lock
	Diagnostics []diagnostic.Diagnostic
}

// Load reads and validates an absolute pawn.lock path. Content problems
// become diagnostics; filesystem failures return an error.
func Load(reg *source.Registry, fsys fsx.FS, path string) (LoadResult, error) {
	content, err := fsys.ReadFile(path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("lockfile: reading %q: %w", path, err)
	}

	fileID := reg.Intern(source.FileURI(path))

	var raw any
	if err := json.Unmarshal(content, &raw); err != nil {
		return LoadResult{
			Diagnostics: []diagnostic.Diagnostic{parseErrorDiagnostic(fileID, content, err)},
		}, nil
	}

	rawMap, ok := raw.(map[string]any)
	if !ok {
		return LoadResult{
			Diagnostics: []diagnostic.Diagnostic{
				newDiagnostic(CodeInvalidType, diagnostic.SeverityError,
					"lockfile document must be a JSON object", wholeFileSpan(fileID, content)),
			},
		}, nil
	}

	var l Lock
	if err := json.Unmarshal(content, &l); err != nil {
		return LoadResult{}, fmt.Errorf("lockfile: decoding %q: %w", path, err)
	}

	l.SourcePath = path

	v := &validator{fileID: fileID, content: content, raw: rawMap, l: &l}
	v.run()

	return LoadResult{Lock: &l, Diagnostics: v.diags}, nil
}

func wholeFileSpan(file source.FileID, content []byte) source.Span {
	span, err := source.NewSpan(file, 0, source.Offset(len(content)))
	if err != nil {
		panic(fmt.Sprintf("lockfile: building whole-file span: %v", err))
	}

	return span
}

func newDiagnostic(code string, severity diagnostic.Severity, message string, span source.Span) diagnostic.Diagnostic {
	return diagnostic.New(code, Source, severity, message, span)
}

func parseErrorDiagnostic(fileID source.FileID, content []byte, err error) diagnostic.Diagnostic {
	span := wholeFileSpan(fileID, content)

	if synErr, ok := err.(*json.SyntaxError); ok { //nolint:errorlint // concrete sentinel type, not wrapped.
		off := source.Offset(synErr.Offset)
		if s, spanErr := source.NewSpan(fileID, off, off); spanErr == nil {
			span = s
		}
	}

	return newDiagnostic(CodeParseError, diagnostic.SeverityError,
		fmt.Sprintf("lockfile failed to parse: %v", err), span)
}
