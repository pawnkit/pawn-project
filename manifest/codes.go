package manifest

// Diagnostic codes emitted by [Load]. Never repurposed once shipped, per
// the shared engineering baseline's diagnostic-code stability rule.
const (
	CodeParseError           = "pawn-project:manifest-parse-error"
	CodeInvalidType          = "pawn-project:manifest-invalid-type"
	CodeInvalidPreset        = "pawn-project:manifest-invalid-preset"
	CodeEmptyEntry           = "pawn-project:manifest-empty-entry"
	CodeEmptyOutput          = "pawn-project:manifest-empty-output"
	CodeInvalidDependency    = "pawn-project:manifest-invalid-dependency"
	CodeSchemaVersionMissing = "pawn-project:manifest-pawnkit-schema-version-missing"
	CodeSchemaVersionInvalid = "pawn-project:manifest-pawnkit-schema-version-unsupported"
	CodeInvalidProfile       = "pawn-project:manifest-invalid-profile"
	CodeUnknownPawnKitField  = "pawn-project:manifest-unknown-pawnkit-field"
	CodePathTraversal        = "pawn-project:manifest-path-traversal"
	CodeMissingBuildName     = "pawn-project:manifest-missing-build-name"
	CodeMissingRuntimeName   = "pawn-project:manifest-missing-runtime-name"
)

// Source is the diagnostic.Source value used for every diagnostic this
// package produces.
const Source = "pawn-project"
