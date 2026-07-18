# Performance

Project loading handles manifests and lockfiles, not Pawn source trees. Most inputs are small, but the work still runs in CLIs and editor startup paths.

## Current expectations

On a warm filesystem cache, loading a normal project should take well under 50 ms. Manifest and lockfile parsing should stay in the low milliseconds. Include resolution grows with the number of include roots because each candidate may require a filesystem check.

These are working expectations, not a published benchmark baseline. The repository does not yet contain `Benchmark` functions.

## Benchmarks to add

Useful fixtures would cover:

- a small project manifest;
- 200 manifest dependencies;
- a 500-package lockfile;
- many include roots and a deep include chain;
- malformed input and dependency cycles.

Once benchmarks exist, compare changes with repeated samples and [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```sh
go test ./... -run=^$ -bench=. -benchmem -count=10 > base.txt
# make the change
go test ./... -run=^$ -bench=. -benchmem -count=10 > new.txt
benchstat base.txt new.txt
```

## Places to watch

Manifest and lockfile loading first decode an untyped shape for validation, then decode the typed model. That costs an extra allocation pass. Archive extraction also holds each bounded entry in memory. Profile before changing either path; simpler code is preferable until measurements show a problem.
