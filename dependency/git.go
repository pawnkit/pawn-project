package dependency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pawnkit/pawn-project/lockfile"
)

const maxGitOutput = 1024 * 1024

// GitInstaller restores Git dependencies at their locked commits.
type GitInstaller struct {
	Command  string
	CacheDir string
	runner   gitRunner
}

// Install restores pkg at its locked commit.
func (g GitInstaller) Install(
	ctx context.Context,
	pkg lockfile.Package,
	target string,
) (Status, error) {
	if pkg.Source.Type != lockfile.SourceTypeGit {
		return "", fmt.Errorf("dependency: source type %q is not supported", pkg.Source.Type)
	}
	if err := validateGitSource(pkg.Source.URL); err != nil {
		return "", err
	}
	if pkg.Commit == "" {
		return "", errors.New("dependency: locked commit is required")
	}
	if !validGitCommit(pkg.Commit) {
		return "", errors.New("dependency: locked commit is invalid")
	}

	command := g.Command
	if command == "" {
		command = "git"
	}
	runner := g.runner
	if runner == nil {
		runner = execGitRunner{}
	}

	targetExists := false
	if _, err := os.Lstat(target); err == nil {
		if err := validateCheckoutDirectory(target); err != nil {
			return "", err
		}
		if status, err := verifyExistingCheckout(ctx, runner, command, pkg, target); err == nil {
			return status, nil
		}
		if err := validateReplaceableCheckout(ctx, runner, command, pkg, target); err != nil {
			return "", err
		}
		targetExists = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("dependency: checking %q: %w", target, err)
	}

	source := pkg.Source.URL
	if g.CacheDir != "" {
		cached, err := ensureCachedCheckout(ctx, runner, command, g.CacheDir, pkg)
		if err != nil {
			return "", err
		}
		source = cached
	}
	if targetExists {
		return replaceCheckout(ctx, runner, command, pkg, source, target)
	}
	return installCheckout(ctx, runner, command, pkg, source, target)
}

// DefaultDependencyCacheDir returns the shared dependency cache directory.
func DefaultDependencyCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pawnkit", "pawn-project", "dependencies"), nil
}

func ensureCachedCheckout(
	ctx context.Context,
	runner gitRunner,
	command, cacheDir string,
	pkg lockfile.Package,
) (string, error) {
	digest := sha256.Sum256([]byte(pkg.Source.URL))
	cache := filepath.Join(cacheDir, hex.EncodeToString(digest[:]), pkg.Commit)
	if _, err := os.Lstat(cache); err == nil {
		if err := validateCheckoutDirectory(cache); err != nil {
			return "", err
		}
		if _, err := verifyExistingCheckout(ctx, runner, command, pkg, cache); err != nil {
			return "", fmt.Errorf("dependency: cached checkout is invalid: %w", err)
		}
		return cache, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("dependency: checking cache %q: %w", cache, err)
	}

	if _, err := installCheckout(ctx, runner, command, pkg, pkg.Source.URL, cache); err != nil {
		if _, statErr := os.Lstat(cache); statErr != nil {
			return "", fmt.Errorf("dependency: caching %s: %w", pkg.Name, err)
		}
		if verifyErr := validateCheckoutDirectory(cache); verifyErr != nil {
			return "", verifyErr
		}
		if _, verifyErr := verifyExistingCheckout(ctx, runner, command, pkg, cache); verifyErr != nil {
			return "", fmt.Errorf("dependency: caching %s: %w", pkg.Name, err)
		}
	}
	return cache, nil
}

func validateCheckoutDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("dependency: checking %q: %w", path, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("dependency: checkout path %q is not a directory", path)
	}
	return nil
}

func validGitCommit(commit string) bool {
	if len(commit) < 7 || len(commit) > 40 {
		return false
	}
	for _, char := range commit {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}

func verifyExistingCheckout(
	ctx context.Context,
	runner gitRunner,
	command string,
	pkg lockfile.Package,
	target string,
) (Status, error) {
	revision, err := runner.Run(ctx, command, "-C", target, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("dependency: existing path %q is not a Git checkout: %w", target, err)
	}
	expected, err := runner.Run(ctx, command, "-C", target, "rev-parse", pkg.Commit+"^{commit}")
	if err != nil || strings.TrimSpace(revision) != strings.TrimSpace(expected) {
		return "", fmt.Errorf(
			"dependency: existing checkout %q is at %s, want %s",
			target,
			strings.TrimSpace(revision),
			pkg.Commit,
		)
	}
	status, err := runner.Run(ctx, command, "-C", target, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", fmt.Errorf("dependency: checking %q status: %w", target, err)
	}
	if strings.TrimSpace(status) != "" {
		return "", fmt.Errorf("dependency: existing checkout %q has local changes", target)
	}
	if err := verifyDirectoryIntegrity(target, pkg.Checksum); err != nil {
		return "", err
	}
	return StatusPresent, nil
}

func validateReplaceableCheckout(
	ctx context.Context,
	runner gitRunner,
	command string,
	pkg lockfile.Package,
	target string,
) error {
	revision, err := runner.Run(ctx, command, "-C", target, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("dependency: existing path %q is not a Git checkout: %w", target, err)
	}
	if strings.TrimSpace(revision) == pkg.Commit {
		return fmt.Errorf("dependency: existing checkout %q failed integrity verification", target)
	}
	status, err := runner.Run(ctx, command, "-C", target, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("dependency: checking %q status: %w", target, err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("dependency: existing checkout %q has local changes", target)
	}
	return nil
}

func replaceCheckout(
	ctx context.Context,
	runner gitRunner,
	command string,
	pkg lockfile.Package,
	source,
	target string,
) (status Status, err error) {
	parent := filepath.Dir(target)
	backup, err := os.MkdirTemp(parent, ".pawnkit-replace-")
	if err != nil {
		return "", fmt.Errorf("dependency: creating replacement backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return "", fmt.Errorf("dependency: preparing replacement backup: %w", err)
	}
	if err := os.Rename(target, backup); err != nil {
		return "", fmt.Errorf("dependency: preserving existing checkout: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(target)
			_ = os.Rename(backup, target)
			return
		}
		_ = os.RemoveAll(backup)
	}()

	return installCheckout(ctx, runner, command, pkg, source, target)
}

func installCheckout(
	ctx context.Context,
	runner gitRunner,
	command string,
	pkg lockfile.Package,
	source,
	target string,
) (Status, error) {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", fmt.Errorf("dependency: creating %q: %w", parent, err)
	}

	staging, err := os.MkdirTemp(parent, ".pawnkit-restore-")
	if err != nil {
		return "", fmt.Errorf("dependency: creating staging directory: %w", err)
	}
	if err := os.Remove(staging); err != nil {
		return "", fmt.Errorf("dependency: preparing staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	if _, err := runner.Run(
		ctx,
		command,
		"clone",
		"--no-checkout",
		"--no-recurse-submodules",
		source,
		staging,
	); err != nil {
		return "", fmt.Errorf("dependency: cloning %s: %w", pkg.Name, err)
	}
	if _, err := runner.Run(ctx, command, "-C", staging, "checkout", "--detach", pkg.Commit); err != nil {
		return "", fmt.Errorf("dependency: checking out %s: %w", pkg.Commit, err)
	}

	revision, err := runner.Run(ctx, command, "-C", staging, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("dependency: verifying %s: %w", pkg.Name, err)
	}
	expected, err := runner.Run(ctx, command, "-C", staging, "rev-parse", pkg.Commit+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("dependency: resolving %s: %w", pkg.Commit, err)
	}
	if strings.TrimSpace(revision) != strings.TrimSpace(expected) {
		return "", fmt.Errorf("dependency: checkout resolved to %s, want %s", strings.TrimSpace(revision), pkg.Commit)
	}
	if err := verifyDirectoryIntegrity(staging, pkg.Checksum); err != nil {
		return "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", fmt.Errorf("dependency: installing %s: %w", pkg.Name, err)
	}

	return StatusInstalled, nil
}

func validateGitSource(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("dependency: parsing source URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("dependency: Git source %q must use HTTPS", raw)
	}
	if parsed.User != nil {
		return errors.New("dependency: Git source URL must not contain credentials")
	}
	return nil
}

type gitRunner interface {
	Run(context.Context, string, ...string) (string, error)
}

type execGitRunner struct{}

func (execGitRunner) Run(ctx context.Context, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...) //nolint:gosec // command is explicit and never uses a shell.
	var output limitedBuffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(output.String())
		if message == "" {
			return "", err
		}
		return "", fmt.Errorf("%w: %s", err, message)
	}
	return output.String(), nil
}

type limitedBuffer struct {
	data bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := maxGitOutput - b.data.Len()
	if remaining > 0 {
		_, _ = b.data.Write(p[:min(len(p), remaining)])
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return b.data.String()
}
