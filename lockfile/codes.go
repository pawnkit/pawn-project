package lockfile

// Diagnostic codes emitted by [Load]. Never repurposed once shipped, per
// the shared engineering baseline's diagnostic-code stability rule.
const (
	CodeParseError             = "pawn-project:lockfile-parse-error"
	CodeInvalidType            = "pawn-project:lockfile-invalid-type"
	CodeSchemaVersionInvalid   = "pawn-project:lockfile-schema-version-unsupported"
	CodeMissingField           = "pawn-project:lockfile-missing-required-field"
	CodeInvalidChecksum        = "pawn-project:lockfile-invalid-checksum"
	CodeInvalidCommit          = "pawn-project:lockfile-invalid-commit"
	CodeInvalidName            = "pawn-project:lockfile-invalid-name"
	CodeInvalidSourceType      = "pawn-project:lockfile-invalid-source-type"
	CodeInvalidKind            = "pawn-project:lockfile-invalid-kind"
	CodeMissingArchiveChecksum = "pawn-project:lockfile-missing-archive-checksum"
	CodeDuplicatePackage       = "pawn-project:lockfile-duplicate-package"
	CodeUnknownDependencyEdge  = "pawn-project:lockfile-unknown-dependency-edge"
	CodeDependencyCycle        = "pawn-project:lockfile-dependency-cycle"
	CodePathTraversal          = "pawn-project:lockfile-path-traversal"
)

// Source is the diagnostic.Source value used for every diagnostic this
// package produces.
const Source = "pawn-project"
