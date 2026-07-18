# Compatibility

## sampctl manifest (`pawn.json`/`pawn.yaml`) field matrix

This table tracks sampctl fields formalized by PawnKit RFC 0002. "Supported" means the field is decoded, validated where appropriate, and available through `manifest.Manifest`.

| Field | Supported | Notes |
|---|---|---|
| `entry` | Yes | minLength:1 enforced (`CodeEmptyEntry`) |
| `output` | Yes | minLength:1 enforced (`CodeEmptyOutput`) |
| `user` / `repo` | Yes | decoded, not otherwise interpreted |
| `dependencies` / `dev_dependencies` | Yes | parsed into structured `Dependency` (scheme, user, repo, ref kind/value); invalid entries reported per-entry, valid entries in the same array still parse |
| `preset` (`samp`/`openmp`) | Yes | validated against the enum; mapped to `samp-037`/`openmp` RFC 0001 profile IDs by `profile.Select` when no explicit override |
| `local` | Yes | decoded; no behavior currently keys off it (no build/run execution in this module) |
| `include_path` | Yes | traversal-checked, resolved via `paths.Resolve` |
| `resources` | Yes (permissive) | decoded as an open map, matching the schema's own permissive modeling (RFC 0002 open question: exact shape not fully specified upstream) |
| `extract_ignore_patterns` | Yes | decoded; not yet consumed by any extraction logic in this module (toolchain archive extraction has its own traversal protection independent of this field) |
| `contributors` | Yes | decoded as `[]any` (string or object form) |
| `website` | Yes | decoded |
| `experimental.build_file` | Yes | drives `paths.Resolved.GeneratedFiles` (defaults to `true` per schema) |
| `build` / `builds[]` | Yes | single or named-array form; `builds[].name` required, enforced |
| `runtime` / `runtimes[]` | Yes | single or named-array form; `runtimes[].name` required, enforced; only fields `pawn-project` itself reasons about are named explicitly (`name`, `version`, `mode`, `port`, `gamemodes`, `filterscripts`, `plugins`); the rest round-trip through `Extra` |
| `pawnkit.schemaVersion` | Yes | required when `pawnkit` present; only `1` currently supported |
| `pawnkit.profile` | Yes | overrides preset-based mapping; pattern-validated |
| `pawnkit.includePaths` | Yes | concatenated with `include_path`, traversal-checked, de-duplicated |
| `pawnkit.tests` | Decoded, not interpreted | `pawntest`'s concern per RFC 0007 |
| `pawnkit.tool` | Yes | consumed by `toolconfig.Resolve`'s merge logic |
| `pawnkit.lockfile` | Yes | defaults to `pawn.lock`, consumed by `project.Load` |

## pawn.lock field matrix

`pawn.lock` has no sampctl precedent (RFC 0003); every field below is a new
PawnKit format, fully supported by `lockfile.Load`.

| Field | Supported |
|---|---|
| `schemaVersion` | Yes; only `1` is supported |
| `generatedAt`, `manifestChecksum` | Yes, including the `manifest-lock-drift` doctor check |
| `compiler.{vendor,version,checksum}` | Yes |
| `runtimeProfile` | Yes |
| `packages[].{name,resolved,version,commit,source,checksum,kind,platformArtifacts,dependencies}` | Yes, including the schema's conditional "archive sources require checksum" rule, checksum/commit/name pattern validation, and dependency-graph cycle/unknown-edge detection |

## Toolchain manager

| Capability (ARCHITECTURE.md) | Status |
|---|---|
| Resolve explicit local compiler first when configured | Implemented |
| Resolve a pinned/downloaded compiler matching the profile | Implemented with verified cache lookup and an explicit artifact URL |
| Store in OS-appropriate cache directory | `toolchain.DefaultCacheDir()` implemented (thin `os.UserCacheDir()` wrapper); resolution logic itself takes any directory, so tests use `t.TempDir()` |
| Verify checksums when published | Implemented (`hash.Content` comparison against `ExpectedChecksum`, both for local paths and downloaded artifacts) |
| Allow offline mode | Implemented (`ResolveOptions.Offline`); `Resolve` returns `ErrOffline` rather than attempting a download |
| Never update silently during a deterministic build | Pinned versions never change; use `Offline` to require cache-only resolution |
| Explicit `update`/`list` operations | Implemented |
| Archive traversal/size protection | Implemented (`toolchain/archive.go`), tested against a hand-built malicious zip and a corrupt-archive fixture |

`HTTPDownloader` accepts HTTPS URLs and an optional `http.Client`. It rejects
URL credentials and redirects away from HTTPS. The default client has a
30-second timeout. Callers provide the artifact URL and checksum, usually from
a lockfile or trusted release index.

## Diagnostic positioning

JSON syntax errors include the byte offset reported by `encoding/json`. Validation errors, such as an invalid preset or dependency string, currently point to the whole file. Precise field ranges require a position-aware JSON and YAML decoder.

## Schema version policy

The manifest extension and lockfile currently have only schema version 1. When `pawnkit-spec` publishes version 2, this module will add another reader and keep version 1 during the migration window.
