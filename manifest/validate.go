package manifest

import (
	"fmt"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/pathutil"
)

type validator struct {
	fileID  source.FileID
	content []byte
	raw     map[string]any
	m       *Manifest
	diags   []diagnostic.Diagnostic
}

func (v *validator) span() source.Span {
	return wholeFileSpan(v.fileID, v.content)
}

func (v *validator) add(code string, severity diagnostic.Severity, format string, args ...any) {
	v.diags = append(v.diags, newDiagnostic(code, severity, fmt.Sprintf(format, args...), v.span()))
}

func (v *validator) run() {
	v.checkPreset()
	v.checkStringMinLength("entry", v.m.Entry, CodeEmptyEntry)
	v.checkStringMinLength("output", v.m.Output, CodeEmptyOutput)
	v.checkDependencies("dependencies", &v.m.Dependencies)
	v.checkDependencies("dev_dependencies", &v.m.DevDependencies)
	v.checkPawnKit()
	v.checkIncludePathTraversal()
	v.checkBuilds()
	v.checkRuntimes()
}

func (v *validator) checkPreset() {
	if v.m.Preset == "" {
		return
	}

	if v.m.Preset != "samp" && v.m.Preset != "openmp" {
		v.add(CodeInvalidPreset, diagnostic.SeverityError,
			`preset %q is not one of "samp", "openmp"`, v.m.Preset)
	}
}

// checkStringMinLength rejects present but empty string fields.
func (v *validator) checkStringMinLength(key, decoded, code string) {
	raw, present := v.raw[key]
	if !present {
		return
	}

	s, ok := raw.(string)
	if !ok {
		v.add(CodeInvalidType, diagnostic.SeverityError, "%q must be a string", key)

		return
	}

	if s == "" {
		v.add(code, diagnostic.SeverityError, "%q must not be empty", key)
	}

	_ = decoded
}

func (v *validator) checkDependencies(key string, out *[]Dependency) {
	raw, present := v.raw[key]
	if !present {
		return
	}

	items, ok := raw.([]any)
	if !ok {
		v.add(CodeInvalidType, diagnostic.SeverityError, "%q must be an array of dependency strings", key)

		return
	}

	parsed := make([]Dependency, 0, len(items))

	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			v.add(CodeInvalidType, diagnostic.SeverityError, "%s[%d] must be a string", key, i)

			continue
		}

		dep, err := ParseDependency(s)
		if err != nil {
			v.add(CodeInvalidDependency, diagnostic.SeverityError, "%s[%d]: %v", key, i, err)

			continue
		}

		parsed = append(parsed, dep)
	}

	*out = parsed
}

func (v *validator) checkPawnKit() {
	rawPK, present := v.raw["pawnkit"]
	if !present {
		return
	}

	pk, ok := rawPK.(map[string]any)
	if !ok {
		v.add(CodeInvalidType, diagnostic.SeverityError, `"pawnkit" must be an object`)

		return
	}

	for key := range pk {
		if !knownPawnKitFields[key] {
			v.add(CodeUnknownPawnKitField, diagnostic.SeverityWarning,
				"pawnkit.%s is not a known PawnKit extension field", key)
		}
	}

	sv, present := pk["schemaVersion"]
	switch {
	case !present:
		v.add(CodeSchemaVersionMissing, diagnostic.SeverityError,
			"pawnkit.schemaVersion is required when the pawnkit object is present")
	default:
		n, ok := sv.(float64)
		if !ok || n != 1 {
			v.add(CodeSchemaVersionInvalid, diagnostic.SeverityError,
				"pawnkit.schemaVersion %v is not supported (this pawn-project understands schema version 1)", sv)
		}
	}

	if v.m.PawnKit != nil && v.m.PawnKit.Profile != "" && !profilePattern.MatchString(v.m.PawnKit.Profile) {
		v.add(CodeInvalidProfile, diagnostic.SeverityError,
			"pawnkit.profile %q does not match ^[a-z][a-z0-9-]*$", v.m.PawnKit.Profile)
	}
}

func (v *validator) checkIncludePathTraversal() {
	for _, p := range v.m.EffectiveIncludePaths() {
		if pathutil.IsAbs(p) || pathutil.HasTraversal(p) {
			v.add(CodePathTraversal, diagnostic.SeverityError,
				"include path %q must be relative to the project root and must not escape it", p)
		}
	}
}

func (v *validator) checkBuilds() {
	for i, b := range v.m.Builds {
		if b.Name == "" {
			v.add(CodeMissingBuildName, diagnostic.SeverityError, "builds[%d] is missing required field %q", i, "name")
		}
	}
}

func (v *validator) checkRuntimes() {
	for i, r := range v.m.Runtimes {
		if r.Name == "" {
			v.add(CodeMissingRuntimeName, diagnostic.SeverityError, "runtimes[%d] is missing required field %q", i, "name")
		}
	}
}
