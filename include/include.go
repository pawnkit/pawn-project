// Package include resolves Pawn include directives.
package include

import (
	"path"
	"sort"
	"strings"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/pathutil"
)

// Resolver searches a fixed, ordered list of include roots.
type Resolver struct {
	fsys        fsx.FS
	roots       []string
	quotedRoots []string
}

// Candidate is an include path offered by an editor.
type Candidate struct {
	Path      string
	Directory bool
}

// New builds a Resolver from ordered, absolute roots.
func New(fsys fsx.FS, roots []string) *Resolver {
	return NewWithQuotedRoots(fsys, roots, nil)
}

// NewWithQuotedRoots adds roots used only by quoted includes.
func NewWithQuotedRoots(fsys fsx.FS, roots, quotedRoots []string) *Resolver {
	cleaned := make([]string, len(roots))
	for i, r := range roots {
		cleaned[i] = pathutil.Clean(r)
	}
	quoted := make([]string, len(quotedRoots))
	for i, r := range quotedRoots {
		quoted[i] = pathutil.Clean(r)
	}

	return &Resolver{fsys: fsys, roots: cleaned, quotedRoots: quoted}
}

// Roots returns the configured search path.
func (r *Resolver) Roots() []string {
	out := make([]string, len(r.roots))
	copy(out, r.roots)

	return out
}

// Complete lists include paths below the typed prefix.
func (r *Resolver) Complete(fromFile, prefix string, quoted bool, limit int) []Candidate {
	if r == nil || limit <= 0 {
		return nil
	}
	limit = min(limit, 200)
	prefix = pathutil.ToSlash(strings.TrimSpace(prefix))
	if strings.HasPrefix(prefix, "/") || strings.Contains(prefix, "../") || prefix == ".." {
		return nil
	}
	directory, partial := path.Split(prefix)
	roots := make([]string, 0, len(r.roots)+len(r.quotedRoots)+1)
	if quoted {
		roots = append(roots, pathutil.Dir(pathutil.Clean(fromFile)))
		roots = append(roots, r.quotedRoots...)
	}
	roots = append(roots, r.roots...)
	seen := make(map[string]bool)
	var candidates []Candidate
	for _, root := range roots {
		base, err := pathutil.SafeJoin(root, directory)
		if err != nil {
			continue
		}
		entries, err := r.fsys.ReadDir(base)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name()) })
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || !strings.HasPrefix(strings.ToLower(name), strings.ToLower(partial)) {
				continue
			}
			candidate := path.Join(directory, name)
			switch {
			case entry.IsDir():
				candidate += "/"
			case strings.EqualFold(path.Ext(candidate), ".inc"):
				candidate = strings.TrimSuffix(candidate, path.Ext(candidate))
			case !strings.EqualFold(path.Ext(candidate), ".pwn"):
				continue
			}
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			candidates = append(candidates, Candidate{Path: candidate, Directory: entry.IsDir()})
			if len(candidates) == limit {
				return candidates
			}
		}
	}
	return candidates
}

// Resolve finds spec from fromFile. Quoted includes search the source
// directory first. Missing extensions are retried with ".inc".
func (r *Resolver) Resolve(fromFile, spec string, quoted bool) (string, bool) {
	matches := r.ResolveAll(fromFile, spec, quoted)
	if len(matches) == 0 {
		return "", false
	}

	return matches[0], true
}

// ResolveAll returns matching include paths in search order.
func (r *Resolver) ResolveAll(fromFile, spec string, quoted bool) []string {
	candidates := searchCandidates(spec)
	normalized := pathutil.ToSlash(strings.TrimSpace(spec))
	var matches []string
	seen := make(map[string]bool)
	add := func(paths ...string) {
		for _, candidate := range paths {
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			matches = append(matches, candidate)
		}
	}
	if strings.HasPrefix(normalized, "./") || strings.HasPrefix(normalized, "../") {
		add(r.relativeMatches(pathutil.Dir(pathutil.Clean(fromFile)), candidates)...)
		return matches
	}

	if quoted {
		for _, root := range r.quotedRoots {
			add(r.dirMatches(root, candidates)...)
		}

		fromDir := pathutil.Dir(pathutil.Clean(fromFile))
		add(r.dirMatches(fromDir, candidates)...)
	}

	for _, root := range r.roots {
		add(r.dirMatches(root, candidates)...)
	}

	return matches
}

func (r *Resolver) relativeMatches(dir string, candidates []string) []string {
	var matches []string
	for _, candidate := range candidates {
		path := pathutil.Join(dir, candidate)
		if r.withinRoot(path) && fsx.IsFile(r.fsys, path) {
			matches = append(matches, path)
		}
	}
	return matches
}

func (r *Resolver) withinRoot(path string) bool {
	path = strings.ToLower(pathutil.Clean(path))
	for _, root := range append(r.roots, r.quotedRoots...) {
		root = strings.ToLower(pathutil.Clean(root))
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

func (r *Resolver) dirMatches(dir string, candidates []string) []string {
	var matches []string
	for _, c := range candidates {
		p, err := pathutil.SafeJoin(dir, c)
		if err != nil {
			continue
		}

		if fsx.IsFile(r.fsys, p) {
			matches = append(matches, p)
		}
	}

	return matches
}

func searchCandidates(spec string) []string {
	spec = pathutil.ToSlash(strings.TrimSpace(spec))

	if extension := strings.ToLower(pathutil.Ext(spec)); extension == ".inc" || extension == ".pwn" {
		return []string{spec}
	}

	return []string{spec, spec + ".inc"}
}
