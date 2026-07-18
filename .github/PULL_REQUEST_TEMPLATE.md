## Summary

<!-- What does this PR change, and why? Link an issue if applicable. -->

## Area

- [ ] New package or public API surface
- [ ] Manifest/lockfile loader behavior (`manifest/`, `lockfile/`)
- [ ] Path/include resolution (`pathutil/`, `paths/`, `include/`, `workspace/`)
- [ ] Toolchain resolver/cache (`toolchain/`)
- [ ] Tool configuration precedence (`toolconfig/`)
- [ ] Doctor checks (`doctor/`)
- [ ] Docs only
- [ ] CI/build only

## Compatibility

- [ ] This is additive; existing APIs and diagnostic codes keep their meaning.
- [ ] This is breaking. I have documented it in `CHANGELOG.md` under
      "Unreleased" and, if it changes manifest/lockfile field handling,
      updated `docs/compatibility.md`.

## Checklist

- [ ] Build, vet, formatting, and race tests pass locally.
- [ ] New diagnostic codes are new codes, not repurposed existing ones.
- [ ] New/changed manifest or lockfile behavior has a hand-written
      invalid-fixture test asserting the specific diagnostic code.
- [ ] Filesystem, network, and clock behavior remains replaceable in tests.
- [ ] No new import of a higher-level PawnKit tool
      (`pawn-analysis`, `pawnlint`, `pawnlsp`, `pawntest`, ...).
