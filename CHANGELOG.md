# Changelog

## 0.16.0 - 2026-07-31

### Fixed

- Resolve resources declared by packages without resource-scheme lock keys.

## 0.15.0 - 2026-07-31

### Added

- Resolve every restored resource package for one host target.

## 0.14.0 - 2026-07-31

### Added

- Inspect selected release assets into complete RFC 0021 lock records.

## 0.13.0 - 2026-07-31

### Added

- Select sampctl resources and release assets deterministically.

## 0.12.0 - 2026-07-31

### Added

- Update validated resource records while preserving sampctl lock data.

## 0.11.0 - 2026-07-31

### Added

- Restore all locked resources for an exact host target as one transaction.

## 0.10.0 - 2026-07-31

### Added

- Install verified resource payloads with staged writes and rollback.

## 0.9.0 - 2026-07-31

### Added

- Download and verify bounded RFC 0021 raw, ZIP, and tar.gz resources.

## 0.8.0 - 2026-07-31

### Added

- Decode and validate experimental RFC 0021 package resource records.

## 0.7.0 - 2026-07-31

### Added

- Expose the accepted sampctl runtime settings to native consumers.

## 0.6.2 - 2026-07-30

### Fixed

- Select and merge sampctl build configurations using sampctl's precedence.
- Resolve sampctl build include paths inside a containing Pawn workspace.

## 0.6.1 - 2026-07-30

### Fixed

- Treat a version-only sampctl compiler setting as pawn-lang.

## 0.6.0 - 2026-07-30

### Fixed

- Resolve sampctl entry and output paths inside a containing Pawn workspace.
- Keep include paths bounded to the manifest directory.

## 0.5.0 - 2026-07-30

### Added

- Install reviewed raw, ZIP, and tar.gz compiler artifacts.
- Verify archive size and the extracted compiler checksum before caching.

## 0.4.0 - 2026-07-30

### Added

- Read checksum-pinned RFC 0019 compiler indexes and select exact targets.

## 0.3.11 - 2026-07-30

### Added

- Expose the compiler coordinate pinned by the lockfile or active build.

## 0.3.10 - 2026-07-30

### Fixed

- Mark cached compiler binaries executable on Unix.

## 0.3.9 - 2026-07-30

### Added

- Find local compilers using the host command search rules.

## 0.3.8 - 2026-07-30

### Fixed

- Decode and validate sampctl resource arrays instead of treating them as objects.

## 0.3.7 - 2026-07-30

### Added

- Restore source checkouts for every sampctl dependency scheme.
- Verify clean checkouts and sampctl-compatible directory integrity.

## 0.3.6 - 2026-07-30

### Fixed

- Use stable lower-case field names when dependency results are encoded as JSON.

## 0.3.5 - 2026-07-30

### Added

- Restore locked Git and local source dependencies without replacing existing checkouts.

## 0.3.4 - 2026-07-30

### Added

- Read sampctl 1.14 `pawn.lock` files and normalize their dependency graph.

## 0.3.3 - 2026-07-30

### Fixed

- Prefer direct dependencies and extracted sampctl resources when resolving includes.

## 0.3.2 - 2026-07-28

### Performance

- Avoid rebuilding paths that are already canonical.

## 0.3.1 - 2026-07-25

### Changed

- Added the repository support record with CI validation.

## 0.3.0 - 2026-07-25

### Changed

- Emit build-backend schema v2 with diagnostic v2 results.

## 0.2.0 - 2026-07-23

### Added

- Added build-backend protocol types and resolved request construction.

## 0.1.10 - 2026-07-23

### Added

- Added canonical manifest creation and JSON encoding.

## 0.1.9 - 2026-07-23

### Fixed

- Matched include completion order to include resolution.

## 0.1.8 - 2026-07-23

### Fixed

- Kept the entry directory first for nested quoted includes.

## 0.1.7 - 2026-07-23

### Added

- Exposed every matching include path in search order.

## 0.1.6 - 2026-07-23

### Added

- Added managed include roots to the project model.
- Added corpus tests for SA-MP and open.mp projects.

## 0.1.5 - 2026-07-23

### Added

- Added bounded include-path candidates for editor completion.

## 0.1.4 - 2026-07-23

### Fixed

- Kept project include roots platform-independent on Windows.

## 0.1.3 - 2026-07-21

- Resolved padded and dotted include names used by existing Pawn projects.

## 0.1.2 - 2026-07-21

- Resolved sampctl build and installed dependency include paths.

All notable changes to this project are documented in this file. The
format loosely follows [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/)
once it reaches 1.0 (pre-1.0, minor versions may include breaking changes,
each documented here).

## [Unreleased]

## [0.1.1] - 2026-07-21

### Fixed

- Resolved nested quoted includes from the manifest entry directory.

## [0.1.0] - 2026-07-18

### Added

- Initial implementation: `fsx`, `pathutil`, `workspace`, `manifest`,
  `lockfile`, `profile`, `paths`, `include`, `toolconfig`, `toolchain`,
  `doctor`, `fingerprint`, and `project` packages.
- Manifest (`pawn.json`/`pawn.yaml`) loading conforming to `pawnkit-spec`'s
  `pawn-project.schema.json` (RFC 0002), with `pawnkit-core/diagnostic`
  diagnostics for malformed or invalid input.
- Lockfile (`pawn.lock`) loading conforming to `pawn-lock.schema.json`
  (RFC 0003), including dependency-graph cycle/unknown-edge detection.
- Deterministic `#include` resolver.
- RFC 0007 tool configuration discovery/merge precedence.
- Toolchain resolver with local and cached resolution, verified HTTPS
  downloads, atomic cache updates, and bounded archive extraction.
- Two initial `doctor` checks (manifest parses, include paths exist).

### Known limitations

- Manifest/lockfile schema-violation diagnostics point at the whole file,
  not the offending key/value (JSON syntax errors do get an exact byte
  offset).
- No legacy-project fallback model yet (GOAL.md success criterion "legacy
  projects can be represented through generated/in-memory defaults and
  diagnostics" — tracked as a follow-up).
- Directory-scoped standalone tool config files (RFC 0007's optional step
  1) are not implemented, only project-root standalone files.
