# Changelog

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
