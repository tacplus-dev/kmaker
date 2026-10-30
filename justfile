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
# The build stamps are not compared: proto_sha is the checkout's HEAD at generation
# time and the compiler stamps (tacplus_*, model/.hash.json) move with every patch
# of the floor's line, so they differ on any later commit or compiler (#7).
# Nor are the compiler's own docs that model/registry.json embeds under "docs"
# (one `"NAME.md":  "…"` line each): they too move with every patch of the
# floor's line (#8, #9; PROPOSALS.md "Reproducible generation").
gates: generate
  #!/usr/bin/env bash
  set -euo pipefail
  git diff --exit-code -I '"(tacplus_version|tacplus_sha|proto_version|proto_sha)"' \
    -I '^    "[A-Z_]+\.md":  "' -- model ':(exclude)model/.hash.json' >&2 \
    || { echo "tracked model/ files differ from a fresh generation; regenerate and commit" >&2; exit 1; }
  unformatted="$(gofmt -l main.go internal)"
  test -z "$unformatted" || { echo "gofmt needed:" >&2; echo "$unformatted" >&2; exit 1; }
  go vet ./...
  go build ./...
  go test ./...
  echo "gates: ok"
