# pawn-project

`pawn-project` gives PawnKit tools one answer to a basic question: which project does this Pawn file belong to, and how is that project built?

It finds the workspace, reads `pawn.json` or `pawn.yaml`, loads `pawn.lock`, resolves include paths, and selects the requested build and runtime profile. The formatter, linter, language server, test runner, and CLI all use the same result.

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

Use `IncludeResolver().Complete` when an editor needs include-path candidates.
The result follows the same root order and path rules as include resolution.

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
