// Package toolconfig loads and merges per-tool configuration.
package toolconfig

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/manifest"
)

// StandaloneFileName returns the config filename for tool.
func StandaloneFileName(tool string) string {
	switch tool {
	case "pawnfmt":
		return ".pawnfmt.json"
	default:
		return tool + ".json"
	}
}

// Resolve merges manifest settings with an optional standalone config.
func Resolve(fsys fsx.FS, root, tool string, m *manifest.Manifest) (map[string]any, error) {
	var standalone map[string]any

	standalonePath := root + "/" + StandaloneFileName(tool)
	if fsx.IsFile(fsys, standalonePath) {
		content, err := fsys.ReadFile(standalonePath)
		if err != nil {
			return nil, fmt.Errorf("toolconfig: reading %q: %w", standalonePath, err)
		}

		if err := json.Unmarshal(content, &standalone); err != nil {
			return nil, fmt.Errorf("toolconfig: parsing %q: %w", standalonePath, err)
		}
	}

	var embedded map[string]any
	if m != nil && m.PawnKit != nil {
		embedded = m.PawnKit.Tool[tool]
	}

	return Merge(standalone, embedded), nil
}

// Merge combines configs. Standalone scalars win; objects recurse and arrays
// are joined with standalone values first.
func Merge(standalone, embedded map[string]any) map[string]any {
	if standalone == nil && embedded == nil {
		return nil
	}

	out := make(map[string]any, len(standalone)+len(embedded))

	maps.Copy(out, embedded)

	for k, sv := range standalone {
		ev, exists := out[k]
		if !exists {
			out[k] = sv

			continue
		}

		out[k] = mergeValue(sv, ev)
	}

	return out
}

func mergeValue(standaloneVal, embeddedVal any) any {
	sMap, sOK := standaloneVal.(map[string]any)
	eMap, eOK := embeddedVal.(map[string]any)

	if sOK && eOK {
		return Merge(sMap, eMap)
	}

	sArr, sOK := standaloneVal.([]any)
	eArr, eOK := embeddedVal.([]any)

	if sOK && eOK {
		return concatDedup(sArr, eArr)
	}

	return standaloneVal
}

func concatDedup(first, second []any) []any {
	out := make([]any, 0, len(first)+len(second))
	seen := make(map[string]bool, len(first)+len(second))

	add := func(items []any) {
		for _, item := range items {
			key := fmt.Sprintf("%#v", item)
			if seen[key] {
				continue
			}

			seen[key] = true

			out = append(out, item)
		}
	}

	add(first)
	add(second)

	return out
}
