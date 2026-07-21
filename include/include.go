// Package include resolves Pawn include directives.
package include

import (
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

// Resolve finds spec from fromFile. Quoted includes search the source
// directory first. Missing extensions are retried with ".inc".
func (r *Resolver) Resolve(fromFile, spec string, quoted bool) (string, bool) {
	candidates := searchCandidates(spec)
	normalized := pathutil.ToSlash(strings.TrimSpace(spec))
	if strings.HasPrefix(normalized, "./") || strings.HasPrefix(normalized, "../") {
		if p, ok := r.tryRelative(pathutil.Dir(pathutil.Clean(fromFile)), candidates); ok {
			return p, true
		}
	}

	if quoted {
		fromDir := pathutil.Dir(pathutil.Clean(fromFile))

		if p, ok := r.tryDir(fromDir, candidates); ok {
			return p, true
		}

		for _, root := range r.quotedRoots {
			if p, ok := r.tryDir(root, candidates); ok {
				return p, true
			}
		}
	}

	for _, root := range r.roots {
		if p, ok := r.tryDir(root, candidates); ok {
			return p, true
		}
	}

	return "", false
}

func (r *Resolver) tryRelative(dir string, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		path := pathutil.Join(dir, candidate)
		if r.withinRoot(path) && fsx.IsFile(r.fsys, path) {
			return path, true
		}
	}
	return "", false
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

func (r *Resolver) tryDir(dir string, candidates []string) (string, bool) {
	for _, c := range candidates {
		p, err := pathutil.SafeJoin(dir, c)
		if err != nil {
			continue
		}

		if fsx.IsFile(r.fsys, p) {
			return p, true
		}
	}

	return "", false
}

func searchCandidates(spec string) []string {
	spec = pathutil.ToSlash(strings.TrimSpace(spec))

	if extension := strings.ToLower(pathutil.Ext(spec)); extension == ".inc" || extension == ".pwn" {
		return []string{spec}
	}

	return []string{spec, spec + ".inc"}
}
