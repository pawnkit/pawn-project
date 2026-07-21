# Architecture

`pawn-project` answers one question for every PawnKit tool: which project owns this file, and how is that project configured?

It finds a workspace, loads its manifest and lockfile, selects a profile, resolves paths, and returns one immutable `Project` value.

## Data flow

```text
file or directory
       |
       v
workspace.FindRoot
       |
       v
manifest.Load ----> diagnostics
       |
       +----> profile.Select
       +----> paths.Resolve ----> include.Resolver
       +----> lockfile.Load
                         |
                         v
                   project.Project
```

Content problems become diagnostics with stable codes. Environment failures, such as an unreadable file, are returned as errors. A manifest or lockfile may still be available when validation reports a problem.

## Packages

| Package | Responsibility |
|---|---|
| `fsx` | Small filesystem interface with OS and in-memory implementations |
| `pathutil` | Platform-independent path cleaning and traversal checks |
| `workspace` | Upward search for `pawn.json` or `pawn.yaml` |
| `manifest` | Project manifest decoding and validation |
| `lockfile` | Lockfile validation and dependency graph |
| `profile` | Build, runtime, and target profile selection |
| `paths` | Absolute entry, output, include, and generated paths |
| `include` | Deterministic include resolution |
| `toolconfig` | Standalone and manifest tool settings |
| `toolchain` | Compiler discovery, verified downloads, and cache management |
| `doctor` | Checks that do not execute project code |
| `fingerprint` | Stable project cache keys |
| `project` | The assembled read-only project model |

Lower-level packages do not import `project`. This lets a consumer use path or manifest handling without pulling in the aggregate model.

## Manifest compatibility

PawnKit does not replace sampctl's manifest. `pawn-project` reads the existing `pawn.json` and `pawn.yaml` fields, including sampctl dependency strings. PawnKit-specific settings live under the optional `pawnkit` object.

This module does not fetch packages or implement `sampctl ensure`. When sampctl has populated `dependencies/`, those package and resource include paths join the project resolver. The field-by-field status is in [compatibility.md](compatibility.md).

## Design choices

### Specifications are data

`pawnkit-spec` publishes JSON Schemas and Markdown, not a Go package. This repository maintains Go types that follow those schemas and tests them against pinned specification examples.

### I/O stays behind small interfaces

Filesystem, download, cache, and clock behavior can be replaced in tests. Project loading and archive tests therefore run offline and do not depend on a user's machine.

### Paths are checked before joining

Manifest, include, lockfile, and archive paths are cleaned and checked for traversal before filesystem access.

### Projects are read-only after loading

`Project` has no setters. Consumers may read one project from multiple goroutines; the test suite checks this under the race detector.

## Ownership

This repository owns project discovery and configuration. It does not own Pawn syntax, semantic analysis, API metadata, or the shared diagnostic model.

| Concern | Owner |
|---|---|
| Parsing | `pawn-parser` |
| Semantic analysis | `pawn-analysis` |
| SA-MP and open.mp API data | `pawn-api` |
| Diagnostics and edits | `pawnkit-core` |
| File-format contracts | `pawnkit-spec` |

## Extension points

- Implement `doctor.Check` to add an environment check.
- Implement `toolchain.Downloader` to use another download transport.
- Add a schema-version branch when `pawnkit-spec` publishes a new manifest or lockfile version. Keep the previous stable reader during migration.
