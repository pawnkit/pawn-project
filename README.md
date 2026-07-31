# pawn-project

[![Maturity: preview](https://img.shields.io/badge/maturity-preview-blue)](.pawnkit/support.json)

`pawn-project` gives PawnKit tools one answer to a basic question: which project does this Pawn file belong to, and how is that project built?

It finds the workspace, reads `pawn.json` or `pawn.yaml`, loads `pawn.lock`, resolves include paths, and selects the requested build and runtime profile. The formatter, linter, language server, test runner, and CLI all use the same result.

The `dependency` package restores dependency sources at the exact Git commits
recorded in `pawn.lock`. The lockfile reader also validates experimental RFC
0021 resource records. `dependency.FetchResource` downloads a locked asset,
checks its archive and file hashes, and returns only the declared files.
`dependency.InstallResourcePayload` stages those files and rolls back the
whole payload if installation fails.
`dependency.ResourceRestorer` selects one exact host target and commits all
of its locked resources together.

## Use it in Go

```sh
go get github.com/pawnkit/pawn-project
```

```go
package main

import (
	"fmt"

	"github.com/pawnkit/pawnkit-core/source"

	"github.com/pawnkit/pawn-project/fsx"
	"github.com/pawnkit/pawn-project/project"
)

func main() {
	reg := source.NewRegistry()

	p, err := project.Load(reg, fsx.OS{}, "/path/to/gamemodes/main.pwn", project.Options{})
	if err != nil {
		panic(err)
	}

	fmt.Println("root:", p.Root())
	fmt.Println("profile:", p.Selection().ProfileID)
	fmt.Println("entry:", p.Paths().Entry)

	for _, d := range p.Diagnostics() {
		fmt.Printf("[%s] %s: %s\n", d.Severity, d.Code, d.Message)
	}
}
```

The loader accepts a project directory or a file inside one. It returns diagnostics for project problems instead of failing at the first bad field. Environment failures, such as an unreadable manifest, are returned as errors.
Nested sampctl manifests may reference entry and output files in a containing
Pawn workspace. Their build include paths may do the same, but PawnKit-specific
include paths remain inside the selected manifest.

Use `IncludeResolver().Complete` when an editor needs include-path candidates.
The result follows the same root order and path rules as include resolution.
Tools can pass absolute managed include directories through
`Options.ManagedIncludeRoots`; they are searched after project dependencies.
Use `Project.BackendRequest` to pass the same resolved profile, paths, defines,
and compiler choice to a build backend.
Use `Project.CompilerCoordinate` to select the compiler pinned by the lockfile
or active build.
Use `toolchain.LoadIndex` to read a checksum-pinned RFC 0019 compiler index.
Selected artifacts carry the archive format, size, executable path, and both
checksums into the resolver.
Use `toolchain.FindCompiler` when a command should honour the host `PATH`.

See [`examples/basic`](examples/basic) for a runnable example.

## Manifest compatibility

`pawn-project` reads sampctl-compatible `pawn.json` and `pawn.yaml` files. PawnKit settings live under the optional `pawnkit` object, so a project does not need to choose between the two tools.

The exact field support and current limitations are listed in [docs/compatibility.md](docs/compatibility.md).

## Status

The module is pre-1.0 and requires Go 1.26 or later. Breaking API changes are recorded in [CHANGELOG.md](CHANGELOG.md).

## Documentation

- [Architecture](docs/architecture.md): package ownership and data flow
- [Compatibility](docs/compatibility.md): manifest fields and schema versions
- [Performance](docs/performance.md): budgets and benchmarks
- [Contributing](CONTRIBUTING.md): local checks and release process
- [Security](SECURITY.md): disclosure and supported versions

## License

[MIT](LICENSE)
