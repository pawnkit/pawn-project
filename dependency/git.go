package dependency

import (
	"bytes"
	"context"
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
	Command string
	runner  gitRunner
}

// Install clones pkg into target without replacing an existing checkout.
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

	command := g.Command
	if command == "" {
		command = "git"
	}
	runner := g.runner
	if runner == nil {
		runner = execGitRunner{}
	}

	if _, err := os.Stat(target); err == nil {
		revision, runErr := runner.Run(ctx, command, "-C", target, "rev-parse", "HEAD")
		if runErr != nil {
			return "", fmt.Errorf("dependency: existing path %q is not a Git checkout: %w", target, runErr)
		}
		expected, runErr := runner.Run(ctx, command, "-C", target, "rev-parse", pkg.Commit+"^{commit}")
		if runErr != nil || strings.TrimSpace(revision) != strings.TrimSpace(expected) {
			return "", fmt.Errorf(
				"dependency: existing checkout %q is at %s, want %s",
				target,
				strings.TrimSpace(revision),
				pkg.Commit,
			)
		}
		return StatusPresent, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("dependency: checking %q: %w", target, err)
	}

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
		pkg.Source.URL,
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
