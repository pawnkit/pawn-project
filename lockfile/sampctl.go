package lockfile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/pathutil"
)

var integrityPattern = regexp.MustCompile(`^(?:sha256:[0-9a-f]{64}|commit:[0-9a-f]{7,40})$`)

type sampctlLock struct {
	Version        int                          `json:"version"`
	Generated      string                       `json:"generated"`
	SampctlVersion string                       `json:"sampctl_version"`
	Dependencies   map[string]sampctlDependency `json:"dependencies"`
	Runtime        *sampctlRuntime              `json:"runtime,omitempty"`
	Build          *sampctlBuild                `json:"build,omitempty"`
	PawnKit        *sampctlPawnKit              `json:"pawnkit,omitempty"`
}

type sampctlPawnKit struct {
	SchemaVersion int                `json:"schema_version"`
	Resources     []ResolvedResource `json:"resources"`
}

type sampctlDependency struct {
	Constraint string   `json:"constraint"`
	Resolved   string   `json:"resolved"`
	Commit     string   `json:"commit"`
	Integrity  string   `json:"integrity,omitempty"`
	Site       string   `json:"site,omitempty"`
	User       string   `json:"user"`
	Repo       string   `json:"repo"`
	Path       string   `json:"path,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Transitive bool     `json:"transitive,omitempty"`
	RequiredBy []string `json:"required_by,omitempty"`
	Scheme     string   `json:"scheme,omitempty"`
	Local      string   `json:"local,omitempty"`
}

type sampctlRuntime struct {
	Version     string `json:"version"`
	Platform    string `json:"platform"`
	RuntimeType string `json:"runtime_type"`
}

type sampctlBuild struct {
	CompilerVersion string `json:"compiler_version,omitempty"`
	CompilerPreset  string `json:"compiler_preset,omitempty"`
}

type diagnosticSink struct {
	span        source.Span
	diagnostics []diagnostic.Diagnostic
}

func (s *diagnosticSink) add(code, message string) {
	s.diagnostics = append(s.diagnostics, newDiagnostic(code, diagnostic.SeverityError, message, s.span))
}

func decodeSampctl(
	fileID source.FileID,
	content []byte,
	raw map[string]any,
) (Lock, []diagnostic.Diagnostic, error) {
	var sourceLock sampctlLock
	if err := json.Unmarshal(content, &sourceLock); err != nil {
		return Lock{}, nil, err
	}

	sink := &diagnosticSink{span: wholeFileSpan(fileID, content)}
	validateSampctlHeader(sourceLock, raw, sink)

	keys := sortedDependencyKeys(sourceLock.Dependencies)
	packages, namesByKey, indexByName := normalizeSampctlPackages(sourceLock.Dependencies, keys, sink)
	linkSampctlDependencies(sourceLock.Dependencies, keys, packages, namesByKey, indexByName, sink)

	lock := Lock{SchemaVersion: 1, GeneratedAt: sourceLock.Generated, Packages: packages}
	if sourceLock.PawnKit != nil {
		if sourceLock.PawnKit.SchemaVersion != 1 {
			sink.add(CodeResourceSchemaInvalid, fmt.Sprintf(
				"pawnkit resource schema version %d is not supported",
				sourceLock.PawnKit.SchemaVersion,
			))
		}
		lock.Resources = sourceLock.PawnKit.Resources
		for i, resource := range lock.Resources {
			if _, ok := sourceLock.Dependencies[resource.Package]; !ok {
				sink.add(CodeUnknownResourcePackage, fmt.Sprintf(
					"resources[%d]: package %q does not match a dependency key",
					i,
					resource.Package,
				))
			}
		}
	}
	if sourceLock.Build != nil && sourceLock.Build.CompilerVersion != "" {
		lock.Compiler = &Compiler{
			Vendor:  compilerVendor(sourceLock.Build.CompilerPreset),
			Version: sourceLock.Build.CompilerVersion,
		}
	}
	if sourceLock.Runtime != nil {
		lock.RuntimeProfile = sourceLock.Runtime.RuntimeType
	}
	return lock, sink.diagnostics, nil
}

func validateSampctlHeader(sourceLock sampctlLock, raw map[string]any, sink *diagnosticSink) {
	if sourceLock.Version != 1 {
		sink.add(CodeSchemaVersionInvalid, fmt.Sprintf(
			"lockfile version %d is not supported (this pawn-project understands sampctl version 1)",
			sourceLock.Version,
		))
	}
	for _, key := range []string{"generated", "sampctl_version", "dependencies"} {
		if _, present := raw[key]; !present {
			sink.add(CodeMissingField, fmt.Sprintf("%q is required", key))
		}
	}
}

func sortedDependencyKeys(dependencies map[string]sampctlDependency) []string {
	keys := make([]string, 0, len(dependencies))
	for key := range dependencies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeSampctlPackages(
	dependencies map[string]sampctlDependency,
	keys []string,
	sink *diagnosticSink,
) ([]Package, map[string]string, map[string]int) {
	packages := make([]Package, 0, len(keys))
	namesByKey := make(map[string]string, len(keys))
	indexByName := make(map[string]int, len(keys))
	for _, key := range keys {
		dependency := dependencies[key]
		name := dependency.User + "/" + dependency.Repo
		namesByKey[key] = name

		kind := dependency.Scheme
		if kind == "" {
			kind = KindDependency
		}
		sourceType := SourceTypeGit
		sourceURL := dependencyURL(dependency)
		if dependency.Local != "" {
			sourceType = SourceTypeLocal
			sourceURL = dependency.Local
			if pathutil.IsAbs(dependency.Local) || pathutil.HasTraversal(dependency.Local) {
				sink.add(CodePathTraversal, fmt.Sprintf("dependency %q local path %q must stay inside the project", key, dependency.Local))
			}
		}

		checksum := ""
		if strings.HasPrefix(dependency.Integrity, "sha256:") {
			checksum = dependency.Integrity
		} else if dependency.Integrity != "" && !integrityPattern.MatchString(dependency.Integrity) {
			sink.add(CodeInvalidIntegrity, fmt.Sprintf("dependency %q integrity %q is invalid", key, dependency.Integrity))
		}

		indexByName[name] = len(packages)
		packages = append(packages, Package{
			Key: key, Constraint: dependency.Constraint, Name: name,
			Resolved: dependency.Resolved, Commit: dependency.Commit,
			Source:   PackageSource{Type: sourceType, URL: sourceURL},
			Checksum: checksum, Integrity: dependency.Integrity, Kind: kind,
			Branch: dependency.Branch, Transitive: dependency.Transitive,
			RequiredBy: append([]string(nil), dependency.RequiredBy...),
		})
	}
	return packages, namesByKey, indexByName
}

func linkSampctlDependencies(
	dependencies map[string]sampctlDependency,
	keys []string,
	packages []Package,
	namesByKey map[string]string,
	indexByName map[string]int,
	sink *diagnosticSink,
) {
	for _, childKey := range keys {
		childName := namesByKey[childKey]
		for _, parentKey := range dependencies[childKey].RequiredBy {
			parentName, ok := namesByKey[parentKey]
			if !ok {
				sink.add(CodeUnknownDependencyEdge, fmt.Sprintf(
					"dependency %q is required by unknown package %q",
					childKey,
					parentKey,
				))
				continue
			}
			parent := &packages[indexByName[parentName]]
			parent.Dependencies = append(parent.Dependencies, childName)
		}
	}
	for i := range packages {
		sort.Strings(packages[i].Dependencies)
	}
}

func dependencyURL(dependency sampctlDependency) string {
	site := dependency.Site
	if site == "" {
		site = "github.com"
	}
	return "https://" + site + "/" + dependency.User + "/" + dependency.Repo
}

func compilerVendor(preset string) string {
	switch strings.ToLower(preset) {
	case "openmp", "open.mp":
		return "openmultiplayer"
	default:
		return "pawn-lang"
	}
}
