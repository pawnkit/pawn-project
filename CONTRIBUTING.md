# Contributing to pawn-project

PawnKit is maintained by volunteers, so reviews may take a little time.

Bug reports, focused fixes, and small project fixtures are welcome. If you are
unsure which package owns a problem, open an issue with the project layout and
the result you expected.

Project discovery sits underneath several PawnKit tools. Check
[docs/architecture.md](docs/architecture.md) before adding a package or public
API, and do not import a higher-level tool such as `pawnlint` or
`pawn-language-server`.

## Local setup

The module requires Go 1.26 or later. Dependencies are resolved through Go modules.

```sh
git clone git@github.com:pawnkit/pawn-project.git
cd pawn-project
go build ./...
go test ./...
```

`pawnkit-spec` is consumed as schemas and examples rather than a Go dependency. Pinned copies of its conformance examples live under `manifest/testdata` and `lockfile/testdata`. Update their provenance file when refreshing them.

## Before a pull request

```sh
go fmt ./...
go build ./...
go vet ./...
CGO_ENABLED=1 go test -race ./...
```

Add a regression test for every bug. Manifest and lockfile validation tests should assert the diagnostic code rather than checking only that some error occurred. Archive changes need a normal fixture and hostile input such as traversal or corruption.

Keep comments short. Explain a surprising constraint or decision, not the name of the function below it.

Diagnostic codes are public behavior. Add a new code for a new condition; do not reuse a published code with a different meaning.

There are no generated files in this repository today. If that changes, document the generator and make CI check for stale output.
