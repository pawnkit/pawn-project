package lockfile

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/pathutil"
)

var (
	checksumPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	commitPattern   = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	namePattern     = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	targetPattern   = regexp.MustCompile(`^[a-z0-9]+-[a-z0-9]+$`)
)

var validSourceTypes = map[string]bool{SourceTypeGit: true, SourceTypeArchive: true, SourceTypeLocal: true}

var validKinds = map[string]bool{
	KindDependency: true, KindDevDependency: true, KindPlugin: true,
	KindComponent: true, KindIncludes: true, KindFilterscript: true,
}

const (
	maxResolvedResources = 1024
	maxResourceFiles     = 4096
	maxResourceBytes     = 1 << 30
)

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
	resourcePackages := make(map[string]bool, len(v.l.Packages)*2)

	for i, p := range v.l.Packages {
		v.checkPackage(i, p)

		if seen[p.Name] {
			v.add(CodeDuplicatePackage, diagnostic.SeverityError, "packages[%d]: duplicate package name %q", i, p.Name)
		}

		seen[p.Name] = true
		resourcePackages[p.Name] = true
		if p.Key != "" {
			resourcePackages[p.Key] = true
		}
	}

	v.checkEdges(seen)
	v.checkResources(resourcePackages)
}

func (v *validator) checkResources(packages map[string]bool) {
	if len(v.l.Resources) > maxResolvedResources {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources: at most %d entries are allowed", maxResolvedResources)
	}
	seen := make(map[string]bool, len(v.l.Resources))
	for i, resource := range v.l.Resources {
		key := resource.Package + "\x00" + resource.Resource + "\x00" + resource.Target
		if seen[key] {
			v.add(CodeDuplicateResource, diagnostic.SeverityError,
				"resources[%d]: duplicate package, resource, and target", i)
		}
		seen[key] = true

		name := strings.TrimPrefix(resource.Package, resourceSchemePrefix(resource.Package))
		if !packages[resource.Package] && !packages[name] {
			v.add(CodeUnknownResourcePackage, diagnostic.SeverityError,
				"resources[%d]: package %q is not in dependencies", i, resource.Package)
		}
		v.checkResource(i, resource)
	}
}

func (v *validator) checkResource(i int, resource ResolvedResource) {
	if !validResourcePackageKey(resource.Package) {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: package key %q is invalid", i, resource.Package)
	}
	if resource.Resource == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `resources[%d]: "resource" is required`, i)
	}
	if !targetPattern.MatchString(resource.Target) {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: target %q is invalid", i, resource.Target)
	}
	if !safeResourceURL(resource.URL) {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: URL must be HTTPS without credentials", i)
	}
	if resource.Size < 1 || resource.Size > maxResourceBytes {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: archive size must be between 1 byte and 1 GiB", i)
	}
	if !checksumPattern.MatchString(resource.Checksum) {
		v.add(CodeInvalidChecksum, diagnostic.SeverityError,
			"resources[%d]: checksum %q is not a sha256:<hex> checksum", i, resource.Checksum)
	}
	if resource.Archive != "zip" && resource.Archive != "tar.gz" && resource.Archive != "file" {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: archive %q is not supported", i, resource.Archive)
	}
	if len(resource.Files) == 0 || len(resource.Files) > maxResourceFiles {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: files must contain between 1 and %d entries", i, maxResourceFiles)
	}
	if resource.Archive == "file" && len(resource.Files) != 1 {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d]: file resources require exactly one file", i)
	}

	destinations := make(map[string]bool, len(resource.Files))
	var extractedBytes int64
	for j, file := range resource.Files {
		v.checkResourceFile(i, j, file, destinations)
		if file.Size > 0 {
			extractedBytes += file.Size
		}
		if extractedBytes > maxResourceBytes {
			v.add(CodeInvalidResource, diagnostic.SeverityError,
				"resources[%d]: extracted files exceed 1 GiB", i)
			break
		}
	}
}

func validResourcePackageKey(key string) bool {
	if prefix := resourceSchemePrefix(key); prefix != "" {
		return namePattern.MatchString(strings.TrimPrefix(key, prefix))
	}
	parts := strings.Split(key, "/")
	return len(parts) == 3 &&
		parts[0] != "" &&
		namePattern.MatchString(parts[1]+"/"+parts[2])
}

func (v *validator) checkResourceFile(resourceIndex, fileIndex int, file ResolvedResourceFile, destinations map[string]bool) {
	for field, path := range map[string]string{"source": file.Source, "destination": file.Destination} {
		if unsafeResourcePath(path) {
			v.add(CodePathTraversal, diagnostic.SeverityError,
				"resources[%d].files[%d]: %s path %q is unsafe", resourceIndex, fileIndex, field, path)
		}
	}
	key := strings.ToLower(pathutil.Clean(file.Destination))
	if destinations[key] {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d].files[%d]: destination %q collides with another file", resourceIndex, fileIndex, file.Destination)
	}
	destinations[key] = true
	if file.Size < 0 || file.Size > maxResourceBytes {
		v.add(CodeInvalidResource, diagnostic.SeverityError,
			"resources[%d].files[%d]: size must be between 0 bytes and 1 GiB", resourceIndex, fileIndex)
	}
	if !checksumPattern.MatchString(file.Checksum) {
		v.add(CodeInvalidChecksum, diagnostic.SeverityError,
			"resources[%d].files[%d]: checksum %q is not a sha256:<hex> checksum",
			resourceIndex, fileIndex, file.Checksum)
	}
}

func resourceSchemePrefix(value string) string {
	for _, prefix := range []string{"plugin://", "component://", "includes://", "filterscript://"} {
		if strings.HasPrefix(value, prefix) {
			return prefix
		}
	}
	return ""
}

func safeResourceURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func unsafeResourcePath(value string) bool {
	return value == "" || value == "." || strings.ContainsRune(value, '\\') ||
		pathutil.IsAbs(value) || pathutil.HasTraversal(value)
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

	if p.Source.Type != SourceTypeLocal && p.Resolved == "" {
		v.add(CodeMissingField, diagnostic.SeverityError, `packages[%d]: "resolved" is required`, i)
	}

	if p.Source.Type == SourceTypeLocal {
		return
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
