package lockfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/pawnkit/pawnkit-core/source"
)

// MarshalSampctlResources updates only the PawnKit resource extension.
func MarshalSampctlResources(
	content []byte,
	lock *Lock,
	resources []ResolvedResource,
) ([]byte, error) {
	if lock == nil {
		return nil, errors.New("lockfile: lock is required")
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("lockfile: decoding source document: %w", err)
	}
	var version int
	if raw, ok := document["version"]; !ok {
		return nil, errors.New("lockfile: source is not a sampctl lockfile")
	} else if err := json.Unmarshal(raw, &version); err != nil || version != 1 {
		return nil, errors.New("lockfile: source is not a sampctl version 1 lockfile")
	}
	if _, ok := document["dependencies"]; !ok {
		return nil, errors.New(`lockfile: source is missing "dependencies"`)
	}

	resources = append([]ResolvedResource(nil), resources...)
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Package != resources[j].Package {
			return resources[i].Package < resources[j].Package
		}
		if resources[i].Resource != resources[j].Resource {
			return resources[i].Resource < resources[j].Resource
		}
		return resources[i].Target < resources[j].Target
	})
	if err := validateResourceUpdate(content, lock, resources); err != nil {
		return nil, err
	}

	extension := make(map[string]json.RawMessage)
	if raw := document["pawnkit"]; len(raw) != 0 {
		if err := json.Unmarshal(raw, &extension); err != nil {
			return nil, fmt.Errorf("lockfile: decoding PawnKit extension: %w", err)
		}
	}
	extension["schema_version"] = json.RawMessage("1")
	resourceJSON, err := json.Marshal(resources)
	if err != nil {
		return nil, fmt.Errorf("lockfile: encoding resources: %w", err)
	}
	if len(resources) == 0 {
		resourceJSON = json.RawMessage("[]")
	}
	extension["resources"] = resourceJSON
	extensionJSON, err := json.Marshal(extension)
	if err != nil {
		return nil, fmt.Errorf("lockfile: encoding PawnKit extension: %w", err)
	}
	document["pawnkit"] = extensionJSON

	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("lockfile: encoding source document: %w", err)
	}
	return append(updated, '\n'), nil
}

func validateResourceUpdate(
	content []byte,
	lock *Lock,
	resources []ResolvedResource,
) error {
	candidate := *lock
	candidate.Resources = resources
	packages := make(map[string]bool, len(candidate.Packages)*2)
	for _, pkg := range candidate.Packages {
		packages[pkg.Name] = true
		if pkg.Key != "" {
			packages[pkg.Key] = true
		}
	}
	registry := source.NewRegistry()
	validator := &validator{
		fileID:  registry.Intern("memory://pawn.lock"),
		content: content,
		l:       &candidate,
	}
	validator.checkResources(packages)
	if len(validator.diags) == 0 {
		return nil
	}
	errs := make([]error, 0, len(validator.diags))
	for _, diag := range validator.diags {
		errs = append(errs, errors.New(diag.Message))
	}
	return fmt.Errorf("lockfile: invalid resource update: %w", errors.Join(errs...))
}
