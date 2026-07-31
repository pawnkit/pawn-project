package dependency

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
	"github.com/pawnkit/pawn-project/manifest"
	"github.com/pawnkit/pawnkit-core/source"
)

const maxDependencyManifestBytes = 1 << 20

const gitHead = "HEAD"

// GitRevisionProvider resolves dependencies through a local Git client.
type GitRevisionProvider struct {
	Command string
	runner  gitRunner
}

// Resolve selects a revision and reads its package manifest.
func (p GitRevisionProvider) Resolve(
	ctx context.Context,
	dep manifest.Dependency,
	locked *lockfile.Package,
) (Revision, error) {
	if dep.Scheme != manifest.SchemeDependency {
		return Revision{}, errors.New("dependency: generic Git providers do not support resource schemes")
	}
	remote := dep.RepositoryURL()
	if err := validateGitSource(remote); err != nil {
		return Revision{}, err
	}
	checkout, err := os.MkdirTemp("", "pawnkit-resolve-")
	if err != nil {
		return Revision{}, fmt.Errorf("dependency: creating resolution directory: %w", err)
	}
	if err := os.Remove(checkout); err != nil {
		return Revision{}, fmt.Errorf("dependency: preparing resolution directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(checkout) }()

	command := p.Command
	if command == "" {
		command = "git"
	}
	runner := p.runner
	if runner == nil {
		runner = execGitRunner{}
	}
	if _, err := runner.Run(ctx, command, "clone", "--no-checkout", "--no-recurse-submodules", remote, checkout); err != nil {
		return Revision{}, fmt.Errorf("dependency: cloning %s: %w", dep.Name(), err)
	}

	ref, resolved, err := selectGitRevision(ctx, runner, command, checkout, dep, locked)
	if err != nil {
		return Revision{}, err
	}
	if !validGitCheckoutRef(ref) {
		return Revision{}, fmt.Errorf("dependency: unsafe Git reference %q", ref)
	}
	if _, err := runner.Run(ctx, command, "-C", checkout, "checkout", "--detach", ref); err != nil {
		return Revision{}, fmt.Errorf("dependency: checking out %s: %w", dep.Name(), err)
	}
	commit, err := runner.Run(ctx, command, "-C", checkout, "rev-parse", gitHead)
	if err != nil {
		return Revision{}, fmt.Errorf("dependency: reading %s revision: %w", dep.Name(), err)
	}
	commit = strings.TrimSpace(commit)
	if len(commit) != 40 || !validGitCommit(commit) {
		return Revision{}, fmt.Errorf("dependency: %s resolved invalid commit %q", dep.Name(), commit)
	}
	if dep.RefKind == manifest.RefCommit {
		resolved = commit[:8]
	}

	packageManifest, err := loadGitRevisionManifest(checkout)
	if err != nil {
		return Revision{}, err
	}
	return Revision{
		Commit: commit, Resolved: resolved,
		CanonicalSite: dep.Site,
		CanonicalName: dep.Name(),
		SourceURL:     remote,
		Manifest:      *packageManifest,
	}, nil
}

func validGitCheckoutRef(ref string) bool {
	return ref != "" && !strings.HasPrefix(ref, "-") &&
		!strings.ContainsAny(ref, "\x00\r\n ~^:?*[\\") &&
		!strings.Contains(ref, "..") && !strings.Contains(ref, "@{") &&
		!strings.HasSuffix(ref, ".") && !strings.HasSuffix(ref, "/")
}

func selectGitRevision(
	ctx context.Context,
	runner gitRunner,
	command, checkout string,
	dep manifest.Dependency,
	locked *lockfile.Package,
) (string, string, error) {
	if locked != nil && locked.Commit != "" {
		return locked.Commit, locked.Resolved, nil
	}
	switch dep.RefKind {
	case manifest.RefNone:
		return gitHead, gitHead, nil
	case manifest.RefBranch:
		return "refs/remotes/origin/" + dep.Ref, dep.Ref, nil
	case manifest.RefCommit:
		return dep.Ref, dep.Ref, nil
	case manifest.RefTag:
		tag := dep.Ref
		if IsTagRange(dep.Ref) {
			output, err := runner.Run(ctx, command, "-C", checkout, "tag", "--list")
			if err != nil {
				return "", "", fmt.Errorf("dependency: listing tags for %s: %w", dep.Name(), err)
			}
			tag, err = SelectTag(strings.Fields(output), dep.Ref)
			if err != nil {
				return "", "", err
			}
		}
		return "refs/tags/" + tag, tag, nil
	default:
		return "", "", fmt.Errorf("dependency: unsupported reference kind %q", dep.RefKind)
	}
}

func loadGitRevisionManifest(checkout string) (*manifest.Manifest, error) {
	for _, name := range []string{"pawn.json", "pawn.yaml", "pawn.yml"} {
		path := filepath.Join(checkout, name)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("dependency: checking package manifest: %w", err)
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, errors.New("dependency: package manifest is not a regular file")
		}
		if info.Size() > maxDependencyManifestBytes {
			return nil, fmt.Errorf("dependency: package manifest exceeds %d bytes", maxDependencyManifestBytes)
		}
		result, err := manifest.Load(source.NewRegistry(), fsx.OS{}, path)
		if err != nil {
			return nil, err
		}
		if result.Manifest == nil || len(result.Diagnostics) != 0 {
			if len(result.Diagnostics) != 0 {
				return nil, fmt.Errorf("dependency: invalid package manifest: %s", result.Diagnostics[0].Message)
			}
			return nil, errors.New("dependency: package manifest did not load")
		}
		return result.Manifest, nil
	}
	return &manifest.Manifest{}, nil
}
