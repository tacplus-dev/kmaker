# Local development builds only - not used in CI/CD.

TACPLUS := env_var_or_default("TACPLUS_BIN", "tacplus")

default: generate

generate:
  #!/usr/bin/env bash
  set -euo pipefail
  {{TACPLUS}} build --tag latest

# What the looper (and a person before pushing) runs: regenerate model/ with the
# floor's compiler and check the tracked part of it matches a fresh generation
# (this repo tracks a subset of model/; generated code is never edited by hand),
# then format, vet, build, test the hand-written code against it.
gates: generate
  #!/usr/bin/env bash
  set -euo pipefail
  stale="$(git status --porcelain -- model)"
  test -z "$stale" || { echo "tracked model/ files differ from a fresh generation; regenerate and commit:" >&2; echo "$stale" >&2; exit 1; }
  unformatted="$(gofmt -l main.go internal)"
  test -z "$unformatted" || { echo "gofmt needed:" >&2; echo "$unformatted" >&2; exit 1; }
  go vet ./...
  go build ./...
  go test ./...
  echo "gates: ok"
