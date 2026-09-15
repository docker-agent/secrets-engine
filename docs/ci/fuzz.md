# Fuzz CI

The `Fuzz` GitHub Actions workflow (`.github/workflows/fuzz.yml`) runs every
Go fuzz target in the repository with a time budget. It calls `make fuzz`,
so CI and a local run share one code path.

## What `make fuzz` does

1. Finds every `_test.go` file outside `vendor/` that declares a
   `func Fuzz...` function.
2. Asks `go test -list '^Fuzz'` for the fuzz targets each package really
   compiles on the current platform. Build tags are honoured.
3. Runs each target on its own with `go test -fuzz '^Name$' -fuzztime $FUZZTIME`.
   Go accepts only one fuzz target per invocation, so the loop is required.
4. Fails if any target fails, and fails if it finds no targets at all.

New fuzz targets need no workflow change. Write the function and CI picks
it up on the next run.

`FUZZTIME` defaults to 30 seconds per target. Override it on the command line:

```sh
make fuzz
make fuzz FUZZTIME=5m
```

To fuzz one target by hand:

```sh
go test -run '^$' -fuzz '^FuzzEncode$' -fuzztime 1m \
  ./store/keychain/internal/go-keychain/secretservice
```

## When the workflow runs

| Trigger | Time per target |
| --- | --- |
| Pull request, push to `main` | 30s |
| Nightly schedule (03:17 UTC) | 10m |
| Manual (`workflow_dispatch`) | `fuzztime` input, default 10m |

Pull requests get a short smoke run so they stay fast. The nightly run does
the deep search.

## Corpus cache

Go stores the inputs it generates under `$(go env GOCACHE)/fuzz`. The
workflow restores the most recent corpus before fuzzing and saves the new
one afterwards, even when the run fails. Each run therefore continues from
the interesting inputs earlier runs found instead of restarting from the
seeds. Cache keys are `fuzz-corpus-<run id>`, and the restore step falls
back to the newest key with the `fuzz-corpus-` prefix.

## Reproducing a failure

When a fuzz target fails, Go writes the crashing input to
`testdata/fuzz/<FuzzName>/<hash>` next to the test file and prints the path.
The workflow uploads that directory as the `fuzz-failing-inputs` artifact.

To reproduce locally:

1. Download the artifact from the failed workflow run.
2. Copy the file into the same `testdata/fuzz/<FuzzName>/` directory in
   your checkout.
3. Run the target as a normal test:

   ```sh
   go test -run 'FuzzEncode/<hash>' ./store/keychain/internal/go-keychain/secretservice
   ```

Files under `testdata/fuzz/<FuzzName>/` are the seed corpus. Plain `go test`
runs them every time, so committing the file turns the crash into a permanent
regression test. Fix the bug and commit the input together.

## Existing targets

| Package | Targets |
| --- | --- |
| `store/keychain/internal/go-keychain/secretservice` | `FuzzEncode`, `FuzzDHExchange` |
