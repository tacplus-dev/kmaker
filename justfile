# Local development builds, and the gates every push to main must pass (the looper's
# manager runs `just gates` before it lands a change). Release builds are CI/CD's.

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
# (one `"NAME.md": "…"` line each; 2.5.3 wrote two spaces after the colon, 2.5.6
# one, so the filter takes either): they too move with every patch of the
# floor's line (#8, #9, #10; PROPOSALS.md "Reproducible generation").
# The generation is forced: tacplus skips it when model/.hash.json matches its
# inputs, even in a clean checkout that lacks the untracked rest of model/. Once
# the tracked part matches, it is restored as committed (its stamps), so the gates
# leave the tracked tree as they found it, as the manager requires (#12).
gates:
  #!/usr/bin/env bash
  set -euo pipefail
  rm -f model/.hash.json
  {{TACPLUS}} build --tag latest
  git diff --exit-code -I '"(tacplus_version|tacplus_sha|proto_version|proto_sha)"' \
    -I '^    "[A-Z_]+\.md": +"' -- model ':(exclude)model/.hash.json' >&2 \
    || { echo "tracked model/ files differ from a fresh generation; regenerate and commit" >&2; exit 1; }
  git checkout -- model
  unformatted="$(gofmt -l main.go internal)"
  test -z "$unformatted" || { echo "gofmt needed:" >&2; echo "$unformatted" >&2; exit 1; }
  go vet ./...
  go build ./...
  go test ./...
  echo "gates: ok"
