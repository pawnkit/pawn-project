package project

import (
	"encoding/json"
	"fmt"

	"github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawn-project/pathutil"
)

// BackendRequest builds a resolved request without reloading the manifest.
func (p *Project) BackendRequest(operation backend.Operation, opts backend.RequestOptions) (backend.Request, error) {
	if !operation.IsValid() {
		return backend.Request{}, backend.ErrInvalidOperation
	}
	if p.selection.ProfileID == "" {
		return backend.Request{}, backend.ErrMissingProfile
	}

	entry, output, arguments, err := p.backendPaths(operation, opts.Output)
	if err != nil {
		return backend.Request{}, err
	}
	defines, err := backendDefines(p.selection.Build)
	if err != nil {
		return backend.Request{}, err
	}

	target := p.selection.ProfileID
	if p.selection.Runtime != nil && p.selection.Runtime.Mode != "" {
		target = p.selection.Runtime.Mode
	}
	compiler := opts.Compiler
	if compiler != nil {
		compilerCopy := *compiler
		if !pathutil.IsAbs(compilerCopy.Path) {
			return backend.Request{}, backend.ErrInvalidCompiler
		}
		if compilerCopy.Version == "" && p.selection.Build != nil && p.selection.Build.Compiler != nil {
			compilerCopy.Version = p.selection.Build.Compiler.Version
		}
		compiler = &compilerCopy
	}

	return backend.Request{
		Kind: "request", SchemaVersion: backend.SchemaVersion, Operation: operation,
		ProjectRoot: p.root, Profile: p.selection.ProfileID, Target: target,
		Entry: entry, Output: output, IncludePaths: append([]string(nil), p.resolved.IncludeRoots...),
		Defines: defines, Compiler: compiler, Arguments: arguments,
	}, nil
}

func (p *Project) backendPaths(operation backend.Operation, outputOverride string) (string, string, []string, error) {
	entry := p.resolved.Entry
	output := p.resolved.Output
	var arguments []string
	if build := p.selection.Build; build != nil {
		var err error
		entry, output, err = resolvedBuildPaths(p.root, build, entry, output)
		if err != nil {
			return "", "", nil, err
		}
		if operation == backend.Build {
			arguments = append([]string(nil), build.Args...)
		}
	}
	if outputOverride != "" {
		if !pathutil.IsAbs(outputOverride) {
			return "", "", nil, backend.ErrInvalidOutput
		}
		output = pathutil.Clean(outputOverride)
	}

	switch operation {
	case backend.Build:
		if entry == "" {
			return "", "", nil, backend.ErrMissingEntry
		}
		if output == "" {
			return "", "", nil, backend.ErrMissingOutput
		}
	case backend.Run:
		if output == "" {
			return "", "", nil, backend.ErrMissingOutput
		}
		entry = ""
	case backend.Restore:
		entry, output = "", ""
	}
	return entry, output, arguments, nil
}

func resolvedBuildPaths(root string, build *manifest.Build, entry, output string) (string, string, error) {
	var err error
	if build.Input != "" {
		entry, err = pathutil.SafeJoin(root, build.Input)
		if err != nil {
			return "", "", err
		}
	}
	if build.Output != "" {
		output, err = pathutil.SafeJoin(root, build.Output)
		if err != nil {
			return "", "", err
		}
	}
	return entry, output, nil
}

func backendDefines(build *manifest.Build) (map[string]string, error) {
	defines := make(map[string]string)
	if build == nil {
		return defines, nil
	}
	for name, value := range build.Constants {
		scalar, err := scalarDefine(value)
		if err != nil {
			return nil, fmt.Errorf("%w for %q", err, name)
		}
		defines[name] = scalar
	}
	return defines, nil
}

func scalarDefine(value any) (string, error) {
	switch value := value.(type) {
	case string:
		return value, nil
	case nil:
		return "", nil
	case bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		raw, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("%w: %w", backend.ErrInvalidDefine, err)
		}
		return string(raw), nil
	default:
		return "", fmt.Errorf("%w: %T", backend.ErrInvalidDefine, value)
	}
}
