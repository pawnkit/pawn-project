# Security policy

Before 1.0, security fixes target the latest released minor version.

## Report a vulnerability

Use GitHub's private [security advisory form](https://github.com/pawnkit/pawn-project/security/advisories/new). If it is unavailable, contact a maintainer listed in `CODEOWNERS` or the PawnKit organization profile.

Include the affected version, likely impact, and a small reproduction when possible. Do not open a public issue before a fix is available.

## Untrusted input

The project loader handles untrusted manifests, lockfiles, include strings, downloaded archives, and toolchain metadata.

- Project paths are checked for traversal before they are joined to a root.
- Include resolution stays inside the source directory or configured include roots.
- Archive extraction rejects absolute and escaping paths and applies file-count and size limits.
- Toolchain downloads have a size limit and must match the expected checksum before entering the cache.
- Cache coordinates and metadata paths cannot escape the cache root.
- Cache updates are staged, so a failed update leaves the previous entry intact.

`HTTPDownloader` rejects URL credentials and redirects away from HTTPS. Its
default client has a 30-second timeout. A supplied client can use a different
timeout or transport policy. Callers remain responsible for trusted artifact
URLs and checksum metadata.

A panic or excessive resource use caused by malformed input is in scope, especially when it can stop a language server or CI job.

This module does not execute Pawn source or AMX bytecode. Runtime issues belong to `goamx`; native plugin isolation belongs to `pawn-plugin-host`.
