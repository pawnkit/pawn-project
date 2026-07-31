package manifest

import (
	"errors"
	"fmt"
	"net/url"
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

const defaultDependencySite = "github.com"

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
	Site    string
	User    string
	Repo    string
	RefKind RefKind
	Ref     string
}

// Name returns the "user/repo" identifier.
func (d Dependency) Name() string {
	return d.User + "/" + d.Repo
}

// RepositoryURL returns the dependency's credential-free HTTPS URL.
func (d Dependency) RepositoryURL() string {
	site := d.Site
	if site == "" {
		site = defaultDependencySite
	}
	return "https://" + site + "/" + d.Name()
}

func dependencyIdentity(d Dependency) string {
	site := d.Site
	if site == "" {
		site = defaultDependencySite
	}
	identity := site + "/" + d.Name()
	if d.Scheme != SchemeDependency {
		identity = string(d.Scheme) + "://" + identity
	}
	return identity
}

// dependencyPattern matches pawn-project.schema.json's $defs.dependencyString.
var dependencyPattern = regexp.MustCompile(
	`^(?:(?:plugin://|component://|includes://|filterscript://)?[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+|https://[A-Za-z0-9.-]+/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)([:#@].+)?$`,
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

	repository, refKind, ref := splitDependencyReference(rest)
	site, user, repo, err := parseDependencyRepository(repository)
	if err != nil {
		return Dependency{}, err
	}
	if scheme != SchemeDependency && site != defaultDependencySite {
		return Dependency{}, errors.New("manifest: prefixed dependencies must use GitHub")
	}

	return Dependency{
		Raw:     raw,
		Scheme:  scheme,
		Site:    site,
		User:    user,
		Repo:    repo,
		RefKind: refKind,
		Ref:     ref,
	}, nil
}

func splitDependencyReference(raw string) (string, RefKind, string) {
	repoStart := strings.IndexByte(raw, '/') + 1
	if strings.HasPrefix(raw, "https://") {
		hostEnd := strings.IndexByte(raw[len("https://"):], '/')
		if hostEnd < 0 {
			return raw, RefNone, ""
		}
		hostEnd += len("https://")
		userEnd := strings.IndexByte(raw[hostEnd+1:], '/')
		if userEnd < 0 {
			return raw, RefNone, ""
		}
		repoStart = hostEnd + 1 + userEnd + 1
	}
	idx := strings.IndexAny(raw[repoStart:], ":@#")
	if idx < 0 {
		return raw, RefNone, ""
	}
	idx += repoStart
	var kind RefKind
	switch raw[idx] {
	case ':':
		kind = RefTag
	case '@':
		kind = RefBranch
	case '#':
		kind = RefCommit
	}
	return raw[:idx], kind, raw[idx+1:]
}

func parseDependencyRepository(raw string) (string, string, string, error) {
	site := defaultDependencySite
	userRepo := raw
	if strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Port() != "" {
			return "", "", "", fmt.Errorf("manifest: dependency URL %q is invalid", raw)
		}
		site = strings.ToLower(parsed.Hostname())
		userRepo = strings.Trim(parsed.EscapedPath(), "/")
	}
	parts := strings.Split(userRepo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("manifest: %q is missing a user/repo separator", raw)
	}
	return site, parts[0], parts[1], nil
}
