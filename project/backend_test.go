package project

import (
	"errors"
	"slices"
	"testing"

	"github.com/pawnkit/pawn-project/backend"
	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawnkit-core/source"
)

func TestBackendRequestUsesResolvedProject(t *testing.T) {
	fsys := fsx.NewMem()
	fsys.AddFile("/project/pawn.json", []byte(`{
		"preset": "openmp",
		"entry": "fallback.pwn",
		"output": "build/fallback.amx",
		"build": {
			"input": "gamemodes/main.pwn",
			"output": "build/main.amx",
			"args": ["-d3"],
			"constants": {"FEATURE": "1", "LIMIT": 10},
			"compiler": {"version": "3.10.10"}
		},
		"runtime": {"mode": "openmp-server"}
	}`))
	fsys.AddFile("/project/gamemodes/main.pwn", nil)

	loaded, err := Load(source.NewRegistry(), fsys, "/project", Options{})
	if err != nil {
		t.Fatal(err)
	}
	request, err := loaded.BackendRequest(backend.Build, backend.RequestOptions{
		Compiler: &backend.Compiler{Path: "/opt/pawncc"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if request.Kind != "request" || request.SchemaVersion != 1 || request.Operation != backend.Build {
		t.Fatalf("identity = %+v", request)
	}
	if request.ProjectRoot != "/project" || request.Profile != "openmp" || request.Target != "openmp-server" {
		t.Fatalf("project selection = %+v", request)
	}
	if request.Entry != "/project/gamemodes/main.pwn" || request.Output != "/project/build/main.amx" {
		t.Fatalf("paths = %+v", request)
	}
	if request.Compiler == nil || request.Compiler.Version != "3.10.10" {
		t.Fatalf("compiler = %+v", request.Compiler)
	}
	if request.Defines["FEATURE"] != "1" || request.Defines["LIMIT"] != "10" {
		t.Fatalf("defines = %v", request.Defines)
	}
	if !slices.Equal(request.Arguments, []string{"-d3"}) {
		t.Fatalf("arguments = %v", request.Arguments)
	}
}

func TestBackendRestoreOmitsBuildPaths(t *testing.T) {
	fsys := fsx.NewMem()
	fsys.AddFile("/project/pawn.json", []byte(`{
		"preset": "openmp",
		"entry": "main.pwn",
		"output": "main.amx"
	}`))

	loaded, err := Load(source.NewRegistry(), fsys, "/project", Options{})
	if err != nil {
		t.Fatal(err)
	}
	request, err := loaded.BackendRequest(backend.Restore, backend.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Entry != "" || request.Output != "" || request.Compiler != nil {
		t.Fatalf("restore request = %+v", request)
	}
}

func TestBackendBuildRequiresOutput(t *testing.T) {
	fsys := fsx.NewMem()
	fsys.AddFile("/project/pawn.json", []byte(`{"preset":"openmp","entry":"main.pwn"}`))

	loaded, err := Load(source.NewRegistry(), fsys, "/project", Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = loaded.BackendRequest(backend.Build, backend.RequestOptions{})
	if !errors.Is(err, backend.ErrMissingOutput) {
		t.Fatalf("error = %v", err)
	}
}

func TestBackendRequestRejectsStructuredDefine(t *testing.T) {
	fsys := fsx.NewMem()
	fsys.AddFile("/project/pawn.json", []byte(`{
		"preset": "openmp",
		"entry": "main.pwn",
		"output": "main.amx",
		"build": {"constants": {"BROKEN": [1, 2]}}
	}`))

	loaded, err := Load(source.NewRegistry(), fsys, "/project", Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = loaded.BackendRequest(backend.Build, backend.RequestOptions{})
	if !errors.Is(err, backend.ErrInvalidDefine) {
		t.Fatalf("error = %v", err)
	}
}
