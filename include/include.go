// Package include resolves Pawn include directives.
package include

import (
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/pathutil"
)

// Resolver searches a fixed, ordered list of include roots.
type Resolver struct {
	fsys  fsx.FS
	roots []string
}

// New builds a Resolver from ordered, absolute roots.
func New(fsys fsx.FS, roots []string) *Resolver {
	cleaned := make([]string, len(roots))
	for i, r := range roots {
		cleaned[i] = pathutil.Clean(r)
	}

	return &Resolver{fsys: fsys, roots: cleaned}
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

	if quoted {
		fromDir := pathutil.Dir(pathutil.Clean(fromFile))

		if p, ok := r.tryDir(fromDir, candidates); ok {
			return p, true
		}
	}

	for _, root := range r.roots {
		if p, ok := r.tryDir(root, candidates); ok {
			return p, true
		}
	}

	return "", false
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
	spec = pathutil.ToSlash(spec)

	if pathutil.Ext(spec) != "" {
		return []string{spec}
	}

	return []string{spec, spec + ".inc"}
}
