package dependency

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/lockfile"
)

type recordingInstaller struct {
	names []string
}

func (i *recordingInstaller) Install(
	_ context.Context,
	pkg lockfile.Package,
	_ string,
) (Status, error) {
	i.names = append(i.names, pkg.Name)
	return StatusInstalled, nil
}

func TestRestoreOrdersDependenciesAndUsesSampctlPaths(t *testing.T) {
	mem := fsx.NewMem()
	installer := &recordingInstaller{}
	lock := &lockfile.Lock{Packages: []lockfile.Package{
		{
			Name: "pawn-lang/YSI-Includes", Commit: "abc", Kind: lockfile.KindDependency,
			Source:       lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/ysi"},
			Dependencies: []string{"openmultiplayer/omp-stdlib"},
		},
		{
			Name: "openmultiplayer/omp-stdlib", Commit: "def", Kind: lockfile.KindIncludes,
			Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/omp"},
		},
	}}

	results, err := NewRestorer(mem, installer).Restore(context.Background(), "/project", lock)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if want := []string{"openmultiplayer/omp-stdlib", "pawn-lang/YSI-Includes"}; !reflect.DeepEqual(installer.names, want) {
		t.Fatalf("install order = %v, want %v", installer.names, want)
	}
	if got, want := results[0].Path, "/project/dependencies/omp-stdlib"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestRestoreAcceptsDependencyCycle(t *testing.T) {
	mem := fsx.NewMem()
	installer := &recordingInstaller{}
	lock := &lockfile.Lock{Packages: []lockfile.Package{
		{
			Name: "owner/a", Commit: "abc", Kind: lockfile.KindDependency,
			Source:       lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/a"},
			Dependencies: []string{"owner/b"},
		},
		{
			Name: "owner/b", Commit: "def", Kind: lockfile.KindDependency,
			Source:       lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/b"},
			Dependencies: []string{"owner/a"},
		},
	}}

	if _, err := NewRestorer(mem, installer).Restore(context.Background(), "/project", lock); err != nil {
		t.Fatal(err)
	}
	if want := []string{"owner/b", "owner/a"}; !reflect.DeepEqual(installer.names, want) {
		t.Fatalf("install order = %v, want %v", installer.names, want)
	}
}

func TestRestoreAcceptsExistingLocalDependency(t *testing.T) {
	mem := fsx.NewMem()
	mem.AddFile("/project/vendor/include/example.inc", nil)
	lock := &lockfile.Lock{Packages: []lockfile.Package{{
		Name: "local/include", Kind: lockfile.KindDependency,
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeLocal, URL: "vendor/include"},
	}}}

	results, err := NewRestorer(mem, &recordingInstaller{}).Restore(context.Background(), "/project", lock)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got, want := results[0].Status, StatusLocal; got != want {
		t.Fatalf("status = %q, want %q", got, want)
	}
}

func TestRestoreRejectsMissingLocalDependency(t *testing.T) {
	lock := &lockfile.Lock{Packages: []lockfile.Package{{
		Name: "local/include", Kind: lockfile.KindDependency,
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeLocal, URL: "missing"},
	}}}

	_, err := NewRestorer(fsx.NewMem(), &recordingInstaller{}).
		Restore(context.Background(), "/project", lock)
	if err == nil {
		t.Fatal("Restore succeeded")
	}
}

func TestRestoreChecksOutResourceSources(t *testing.T) {
	lock := &lockfile.Lock{Packages: []lockfile.Package{{
		Name: "samp-incognito/streamer", Kind: lockfile.KindPlugin,
		Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/streamer"},
	}}}

	results, err := NewRestorer(fsx.NewMem(), &recordingInstaller{}).
		Restore(context.Background(), "/project", lock)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got, want := results[0].Path, "/project/dependencies/streamer"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestRestoreRejectsCollidingRepositoryNames(t *testing.T) {
	lock := &lockfile.Lock{Packages: []lockfile.Package{
		{
			Name: "one/common", Kind: lockfile.KindDependency,
			Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/one"},
		},
		{
			Name: "two/common", Kind: lockfile.KindDependency,
			Source: lockfile.PackageSource{Type: lockfile.SourceTypeGit, URL: "https://example.com/two"},
		},
	}}

	_, err := NewRestorer(fsx.NewMem(), &recordingInstaller{}).
		Restore(context.Background(), "/project", lock)
	if err == nil {
		t.Fatal("Restore succeeded")
	}
}

func TestRestoreHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewRestorer(fsx.NewMem(), &recordingInstaller{}).
		Restore(ctx, "/project", &lockfile.Lock{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
