package lockfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pawnkit/pawnkit-core/diagnostic"
	"github.com/pawnkit/pawnkit-core/source"
)

const sampctlCompatibilityVersion = "1.14.1"

// MarshalSampctlDependencies writes a complete sampctl dependency object.
func MarshalSampctlDependencies(
	content []byte,
	packages []Package,
	generated time.Time,
) ([]byte, error) {
	document, previous, err := dependencyDocument(content)
	if err != nil {
		return nil, err
	}
	dependencies, err := encodeSampctlDependencies(packages)
	if err != nil {
		return nil, err
	}

	document["version"] = json.RawMessage("1")
	generatedJSON, _ := json.Marshal(generated.UTC().Format(time.RFC3339Nano))
	document["generated"] = generatedJSON
	if _, ok := document["sampctl_version"]; !ok {
		versionJSON, _ := json.Marshal(sampctlCompatibilityVersion)
		document["sampctl_version"] = versionJSON
	}
	dependencyJSON, err := json.Marshal(dependencies)
	if err != nil {
		return nil, fmt.Errorf("lockfile: encoding dependencies: %w", err)
	}
	document["dependencies"] = dependencyJSON
	if err := filterChangedResourceRecords(document, previous, dependencies); err != nil {
		return nil, err
	}

	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("lockfile: encoding source document: %w", err)
	}
	return append(updated, '\n'), nil
}

func dependencyDocument(
	content []byte,
) (map[string]json.RawMessage, map[string]sampctlDependency, error) {
	if len(strings.TrimSpace(string(content))) == 0 {
		return make(map[string]json.RawMessage), nil, nil
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, nil, fmt.Errorf("lockfile: decoding source document: %w", err)
	}
	var version int
	if raw, ok := document["version"]; !ok {
		return nil, nil, errors.New("lockfile: source is not a sampctl lockfile")
	} else if err := json.Unmarshal(raw, &version); err != nil || version != 1 {
		return nil, nil, errors.New("lockfile: source is not a sampctl version 1 lockfile")
	}
	var dependencies map[string]sampctlDependency
	if raw := document["dependencies"]; len(raw) != 0 {
		if err := json.Unmarshal(raw, &dependencies); err != nil {
			return nil, nil, fmt.Errorf("lockfile: decoding dependencies: %w", err)
		}
	}
	return document, dependencies, nil
}

func encodeSampctlDependencies(packages []Package) (map[string]sampctlDependency, error) {
	validator := &validator{
		fileID: source.NewRegistry().Intern("memory://pawn.lock"),
		l:      &Lock{SchemaVersion: 1, Packages: packages},
	}
	dependencies := make(map[string]sampctlDependency, len(packages))
	for i, pkg := range packages {
		validator.checkPackage(i, pkg)
		if !validResourcePackageKey(pkg.Key) {
			validator.add(CodeInvalidName, diagnostic.SeverityError,
				"packages[%d]: dependency key %q is invalid", i, pkg.Key)
		}
		if _, exists := dependencies[pkg.Key]; exists {
			validator.add(CodeDuplicatePackage, diagnostic.SeverityError,
				"packages[%d]: duplicate dependency key %q", i, pkg.Key)
			continue
		}
		dependency, err := encodeSampctlDependency(pkg)
		if err != nil {
			return nil, err
		}
		dependencies[pkg.Key] = dependency
	}
	if len(validator.diags) != 0 {
		errs := make([]error, 0, len(validator.diags))
		for _, diag := range validator.diags {
			errs = append(errs, errors.New(diag.Message))
		}
		return nil, fmt.Errorf("lockfile: invalid dependency update: %w", errors.Join(errs...))
	}
	return dependencies, nil
}

func encodeSampctlDependency(pkg Package) (sampctlDependency, error) {
	user, repo, ok := strings.Cut(pkg.Name, "/")
	if !ok {
		return sampctlDependency{}, fmt.Errorf("lockfile: invalid package name %q", pkg.Name)
	}
	dependency := sampctlDependency{
		Constraint: pkg.Constraint,
		Resolved:   pkg.Resolved,
		Commit:     pkg.Commit,
		Integrity:  pkg.Integrity,
		User:       user,
		Repo:       repo,
		Path:       pkg.Path,
		Branch:     pkg.Branch,
		Transitive: pkg.Transitive,
		RequiredBy: append([]string(nil), pkg.RequiredBy...),
	}
	if dependency.Integrity == "" && pkg.Commit != "" {
		dependency.Integrity = "commit:" + pkg.Commit
	}
	if pkg.Source.Type == SourceTypeLocal {
		dependency.Local = pkg.Source.URL
	} else {
		parsed, err := url.Parse(pkg.Source.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return sampctlDependency{}, fmt.Errorf("lockfile: package %q has unsafe source URL", pkg.Name)
		}
		dependency.Site = parsed.Host
	}
	if validKinds[pkg.Kind] && pkg.Kind != KindDependency && pkg.Kind != KindDevDependency {
		dependency.Scheme = pkg.Kind
	}
	sort.Strings(dependency.RequiredBy)
	return dependency, nil
}

func filterChangedResourceRecords(
	document map[string]json.RawMessage,
	previous map[string]sampctlDependency,
	current map[string]sampctlDependency,
) error {
	raw := document["pawnkit"]
	if len(raw) == 0 {
		return nil
	}
	var extension map[string]json.RawMessage
	if err := json.Unmarshal(raw, &extension); err != nil {
		return fmt.Errorf("lockfile: decoding PawnKit extension: %w", err)
	}
	var resources []ResolvedResource
	if rawResources := extension["resources"]; len(rawResources) != 0 {
		if err := json.Unmarshal(rawResources, &resources); err != nil {
			return fmt.Errorf("lockfile: decoding resolved resources: %w", err)
		}
	}
	kept := resources[:0]
	for _, resource := range resources {
		before, hadBefore := previous[resource.Package]
		after, hasAfter := current[resource.Package]
		if hadBefore && hasAfter && before.Commit != "" && before.Commit == after.Commit {
			kept = append(kept, resource)
		}
	}
	resourceJSON, err := json.Marshal(kept)
	if err != nil {
		return fmt.Errorf("lockfile: encoding resolved resources: %w", err)
	}
	extension["resources"] = resourceJSON
	extensionJSON, err := json.Marshal(extension)
	if err != nil {
		return fmt.Errorf("lockfile: encoding PawnKit extension: %w", err)
	}
	document["pawnkit"] = extensionJSON
	return nil
}
