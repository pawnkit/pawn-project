# Compatibility

## sampctl manifest (`pawn.json`/`pawn.yaml`) field matrix

This table tracks sampctl fields formalized by PawnKit RFC 0002. "Supported" means the field is decoded, validated where appropriate, and available through `manifest.Manifest`.

| Field | Supported | Notes |
|---|---|---|
| `entry` | Yes | May resolve into a containing Pawn workspace, matching nested sampctl projects |
| `output` | Yes | May resolve into a containing Pawn workspace; paths outside it are rejected |
| `user` / `repo` | Yes | decoded, not otherwise interpreted |
| `dependencies` / `dev_dependencies` | Yes | parsed into structured `Dependency` (scheme, user, repo, ref kind/value); invalid entries reported per-entry, valid entries in the same array still parse |
| `preset` (`samp`/`openmp`) | Yes | validated against the enum; mapped to `samp-037`/`openmp` RFC 0001 profile IDs by `profile.Select` when no explicit override |
| `local` | Yes | decoded; no behavior currently keys off it (no build/run execution in this module) |
| `include_path` | Yes | traversal-checked, resolved via `paths.Resolve` |
| `resources` | Yes | sampctl's resource array, including archives, includes, plugins, and file mappings |
| `extract_ignore_patterns` | Yes | decoded; not yet consumed by any extraction logic in this module (toolchain archive extraction has its own traversal protection independent of this field) |
| `contributors` | Yes | decoded as `[]any` (string or object form) |
| `website` | Yes | decoded |
| `experimental.build_file` | Yes | drives `paths.Resolved.GeneratedFiles` (defaults to `true` per schema) |
| `build` / `builds[]` | Yes | single or named-array form; `builds[].name` required; active build include paths are resolved |
| `runtime` / `runtimes[]` | Yes | single or named-array form; `runtimes[].name` required, enforced; only fields `pawn-project` itself reasons about are named explicitly (`name`, `version`, `mode`, `port`, `gamemodes`, `filterscripts`, `plugins`); the rest round-trip through `Extra` |
| `pawnkit.schemaVersion` | Yes | required when `pawnkit` present; only `1` currently supported |
| `pawnkit.profile` | Yes | overrides preset-based mapping; pattern-validated |
| `pawnkit.includePaths` | Yes | concatenated with `include_path`, traversal-checked, de-duplicated |
| `pawnkit.tests` | Decoded, not interpreted | `pawntest`'s concern per RFC 0007 |
| `pawnkit.tool` | Yes | consumed by `toolconfig.Resolve`'s merge logic |
| `pawnkit.lockfile` | Yes | defaults to `pawn.lock`, consumed by `project.Load` |

## pawn.lock field matrix

`lockfile.Load` reads sampctl 1.14 version 1 lockfiles and normalizes them into
PawnKit's package graph. It also reads the earlier PawnKit draft during the
RFC 0003 migration window.

| Field | Supported |
|---|---|
| `version` | Yes; only sampctl version `1` is supported |
| `generated`, `sampctl_version` | Yes |
| `dependencies` | Yes, including constraints, revisions, integrity, schemes, local paths, and reverse edges |
| `runtime` | Runtime type is exposed as the normalized runtime profile |
| `build` | Compiler version and preset are exposed through the normalized compiler |
| `pawnkit.resources` | Experimental RFC 0021 records are decoded, validated, downloaded, verified, and installed as a staged transaction |
| Earlier `schemaVersion` / `packages` draft | Read-only compatibility through 2027-07-30 |

## Dependency restore

`dependency.Restorer` installs Git dependencies at the commit recorded in
`pawn.lock`. Existing checkouts must already match that commit. Local paths are
checked but not copied.

All remote schemes use the same source checkout layout. RFC 0021 resource
records are checked for exact package keys, target coordinates, HTTPS URLs,
checksums, bounded sizes, and safe destination paths. ZIP, tar.gz, and raw
assets can be downloaded and verified in memory. Filesystem installation is
staged below `.pawnkit` and rolled back as one payload when a write fails.
All locked records for an exact host target are verified before that payload
is installed. PawnKit can update the namespaced resource set without changing
sampctl dependency data. Manifest selection follows sampctl's exact-version,
then platform-default precedence. Release archive inspection remains to be
implemented.

## Toolchain manager

| Capability (ARCHITECTURE.md) | Status |
|---|---|
| Resolve explicit local compiler first when configured | Implemented |
| Resolve a pinned/downloaded compiler matching the profile | Implemented with verified cache lookup and an explicit artifact URL |
| Select the compiler pinned by the lockfile or active build | Implemented through `Project.CompilerCoordinate` |
| Read a reviewed compiler artifact index | Implemented for RFC 0019 schema version 1 |
| Store in OS-appropriate cache directory | `toolchain.DefaultCacheDir()` implemented (thin `os.UserCacheDir()` wrapper); resolution logic itself takes any directory, so tests use `t.TempDir()` |
| Verify checksums when published | Implemented (`hash.Content` comparison against `ExpectedChecksum`, both for local paths and downloaded artifacts) |
| Allow offline mode | Implemented (`ResolveOptions.Offline`); `Resolve` returns `ErrOffline` rather than attempting a download |
| Never update silently during a deterministic build | Pinned versions never change; use `Offline` to require cache-only resolution |
| Explicit `update`/`list` operations | Implemented |
| Reviewed compiler archives | Raw, ZIP, and tar.gz artifacts; exact executable path and checksum verified before caching |
| Archive traversal/size protection | Implemented for ZIP and tar.gz archives, including entry-count and extracted-size limits |

`HTTPDownloader` accepts HTTPS URLs and an optional `http.Client`. It rejects
URL credentials and redirects away from HTTPS. The default client has a
30-second timeout. Callers provide the artifact URL and checksum, usually from
a lockfile or trusted release index.

## Diagnostic positioning

JSON syntax errors include the byte offset reported by `encoding/json`. Validation errors, such as an invalid preset or dependency string, currently point to the whole file. Precise field ranges require a position-aware JSON and YAML decoder.

## Schema version policy

The manifest extension uses schema version 1. The lockfile follows sampctl's
version field. A new version gets a separate reader while the supported
migration window remains open.
