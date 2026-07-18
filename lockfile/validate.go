package lockfile

import (
	"fmt"
	"regexp"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/pathutil"
)

var (
	checksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	commitPattern   = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	namePattern     = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

var validSourceTypes = map[string]bool{SourceTypeGit: true, SourceTypeArchive: true, SourceTypeLocal: true}

var validKinds = map[string]bool{
	KindDependency: true, KindDevDependency: true, KindPlugin: true,
	KindComponent: true, KindIncludes: true, KindFilterscript: true,
}

type validator struct {
	fileID  source.FileID
	content []byte
	raw     map[string]any
	l       *Lock
	diags   []diagnostic.Diagnostic
}

func (v *validator) span() source.Span {
	return wholeFileSpan(v.fileID, v.content)
}

func (v *validator) add(code string, severity diagnostic.Severity, format string, args ...any) {
	v.diags = append(v.diags, newDiagnostic(code, severity, fmt.Sprintf(format, args...), v.span()))
}

func (v *validator) run() {
	if v.l.SchemaVersion != 1 {
		v.add(CodeSchemaVersionInvalid, diagnostic.SeverityError,
			"schemaVersion %d is not supported (this pawn-project understands schema version 1)", v.l.SchemaVersion)
	}

	if _, present := v.raw["packages"]; !present {
		v.add(CodeMissingField, diagnostic.SeverityError, `"packages" is required`)
	}

	if v.l.ManifestChecksum != "" && !checksumPattern.MatchString(v.l.ManifestChecksum) {
		v.add(CodeInvalidChecksum, diagnostic.SeverityError, "manifestChecksum %q is not a sha256:<hex> checksum", v.l.ManifestChecksum)
	}

	if v.l.Compiler != nil && v.l.Compiler.Checksum != "" && !checksumPattern.MatchString(v.l.Compiler.Checksum) {
		v.add(CodeInvalidChecksum, diagnostic.SeverityError, "compiler.checksum %q is not a sha256:<hex> checksum", v.l.Compiler.Checksum)
	}

	seen := make(map[string]bool, len(v.l.Packages))

	for i, p := range v.l.Packages {
		v.checkPackage(i, p)

		if seen[p.Name] {
			v.add(CodeDuplicatePackage, diagnostic.SeverityError, "packages[%d]: duplicate package name %q", i, p.Name)
		}

		seen[p.Name] = true
	}

	v.checkEdges(seen)
	v.checkCycles()
}

func (v *validator) checkPackage(i int, p Package) {
	v.checkPackageIdentity(i, p)
	v.checkPackageSource(i, p)
	v.checkPackageChecksum(i, p)

	for j, a := range p.PlatformArtifacts {
		v.checkPlatformArtifact(i, j, a)
	}
}

func (v *validator) checkPackageIdentity(i int, p Package) {
	if p.Name == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "name" is required`, i)
	} else if !namePattern.MatchString(p.Name) {
		v.add(CodeInvalidName, diagnostic.SeverityError, "packages[%d]: name %q must match user/repo", i, p.Name)
	}

	if p.Resolved == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "resolved" is required`, i)
	}

	if p.Commit == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "commit" is required`, i)
	} else if !commitPattern.MatchString(p.Commit) {
		v.add(CodeInvalidCommit, diagnostic.SeverityError, "packages[%d]: commit %q must be 7-40 lowercase hex characters", i, p.Commit)
	}

	if p.Kind == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "kind" is required`, i)
	} else if !validKinds[p.Kind] {
		v.add(CodeInvalidKind, diagnostic.SeverityError, "packages[%d]: kind %q is not a recognized kind", i, p.Kind)
	}
}

func (v *validator) checkPackageSource(i int, p Package) {
	if p.Source.Type == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "source.type" is required`, i)
	} else if !validSourceTypes[p.Source.Type] {
		v.add(CodeInvalidSourceType, diagnostic.SeverityError, "packages[%d]: source.type %q must be git, archive, or local", i, p.Source.Type)
	}

	if p.Source.URL == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "source.url" is required`, i)
	}
}

func (v *validator) checkPackageChecksum(i int, p Package) {
	if p.Checksum != "" && !checksumPattern.MatchString(p.Checksum) {
		v.add(CodeInvalidChecksum, diagnostic.SeverityError, "packages[%d]: checksum %q is not a sha256:<hex> checksum", i, p.Checksum)
	}

	if p.Source.Type == SourceTypeArchive && p.Checksum == "" {
		v.add(CodeMissingArchiveChecksum, diagnostic.SeverityError,
			"packages[%d]: archive sources require a checksum", i)
	}
}

func (v *validator) checkPlatformArtifact(i, j int, a PlatformArtifact) {
	if a.Checksum != "" && !checksumPattern.MatchString(a.Checksum) {
		v.add(CodeInvalidChecksum, diagnostic.SeverityError,
			"packages[%d].platformArtifacts[%d]: checksum %q is not a sha256:<hex> checksum", i, j, a.Checksum)
	}

	if a.Path != "" && (pathutil.IsAbs(a.Path) || pathutil.HasTraversal(a.Path)) {
		v.add(CodePathTraversal, diagnostic.SeverityError,
			"packages[%d].platformArtifacts[%d]: path %q must be relative and must not escape the extraction root", i, j, a.Path)
	}
}

func (v *validator) checkEdges(known map[string]bool) {
	for i, p := range v.l.Packages {
		for _, dep := range p.Dependencies {
			if !known[dep] {
				v.add(CodeUnknownDependencyEdge, diagnostic.SeverityError,
					"packages[%d] (%s): dependency edge to unknown package %q", i, p.Name, dep)
			}
		}
	}
}

// checkCycles detects cycles in the resolved dependency graph without
// panicking or infinite-looping on malicious/malformed input.
func (v *validator) checkCycles() {
	const (
		white = 0
		gray  = 1
		black = 2
	)

	byName := make(map[string]Package, len(v.l.Packages))
	for _, p := range v.l.Packages {
		byName[p.Name] = p
	}

	state := make(map[string]int, len(v.l.Packages))

	var visit func(name string, path []string) []string

	visit = func(name string, path []string) []string {
		if state[name] == black {
			return nil
		}

		if state[name] == gray {
			return append(path, name)
		}

		state[name] = gray
		path = append(path, name)

		for _, dep := range byName[name].Dependencies {
			if _, ok := byName[dep]; !ok {
				continue // already reported by checkEdges
			}

			if cyc := visit(dep, path); cyc != nil {
				return cyc
			}
		}

		state[name] = black

		return nil
	}

	reported := make(map[string]bool)

	for _, p := range v.l.Packages {
		if state[p.Name] != white {
			continue
		}

		if cyc := visit(p.Name, nil); cyc != nil {
			key := fmt.Sprint(cyc)
			if !reported[key] {
				v.add(CodeDependencyCycle, diagnostic.SeverityError, "dependency cycle detected: %v", cyc)
				reported[key] = true
			}
		}
	}
}
