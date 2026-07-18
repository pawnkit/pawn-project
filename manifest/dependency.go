package manifest

import (
	"fmt"
	"regexp"
	"strings"
)

// Scheme selects a dependency's install target, per RFC 0002's documented
// sampctl prefix schemes.
type Scheme string

const (
	SchemeDependency   Scheme = ""
	SchemePlugin       Scheme = "plugin"
	SchemeComponent    Scheme = "component"
	SchemeIncludes     Scheme = "includes"
	SchemeFilterscript Scheme = "filterscript"
)

// RefKind identifies how a dependency string pins a version.
type RefKind string

const (
	RefNone   RefKind = ""
	RefTag    RefKind = "tag"    // user/repo:1.2.3
	RefBranch RefKind = "branch" // user/repo@branch-name
	RefCommit RefKind = "commit" // user/repo#sha1
)

// Dependency is a parsed sampctl dependency reference.
type Dependency struct {
	Raw     string
	Scheme  Scheme
	User    string
	Repo    string
	RefKind RefKind
	Ref     string
}

// Name returns the "user/repo" identifier.
func (d Dependency) Name() string {
	return d.User + "/" + d.Repo
}

// dependencyPattern matches pawn-project.schema.json's $defs.dependencyString.
var dependencyPattern = regexp.MustCompile(
	`^(plugin://|component://|includes://|filterscript://)?[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+([:#@].+)?$`,
)

var schemePrefixes = []struct {
	prefix string
	scheme Scheme
}{
	{"plugin://", SchemePlugin},
	{"component://", SchemeComponent},
	{"includes://", SchemeIncludes},
	{"filterscript://", SchemeFilterscript},
}

// ParseDependency parses a sampctl dependency reference.
func ParseDependency(raw string) (Dependency, error) {
	if !dependencyPattern.MatchString(raw) {
		return Dependency{}, fmt.Errorf("manifest: %q does not match the dependency string pattern", raw)
	}

	rest := raw
	scheme := SchemeDependency

	for _, sp := range schemePrefixes {
		if strings.HasPrefix(rest, sp.prefix) {
			scheme = sp.scheme
			rest = rest[len(sp.prefix):]

			break
		}
	}

	userRepo := rest
	refKind := RefNone

	var ref string

	if idx := strings.IndexAny(rest, ":@#"); idx >= 0 {
		userRepo = rest[:idx]
		ref = rest[idx+1:]

		switch rest[idx] {
		case ':':
			refKind = RefTag
		case '@':
			refKind = RefBranch
		case '#':
			refKind = RefCommit
		}
	}

	before, after, ok := strings.Cut(userRepo, "/")
	if !ok {
		return Dependency{}, fmt.Errorf("manifest: %q is missing a user/repo separator", raw)
	}

	return Dependency{
		Raw:     raw,
		Scheme:  scheme,
		User:    before,
		Repo:    after,
		RefKind: refKind,
		Ref:     ref,
	}, nil
}
