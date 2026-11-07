# Proposals: kmaker

Written by loopers. Each proposal: what, why, links. Jody promotes accepted ones into ROADMAP.md (via an agent);
the looper then marks it `promoted` or `declined` here with the date.

## Open
- **<title>** (<date>, <issue/run link>): <what and why>
- **Reproducible generation: no build stamps in generated code** (2026-10-28, [#7](https://github.com/tacplus-dev/kmaker/issues/7)):
  tacplus writes the checkout's git `HEAD` (`proto_sha`) and the compiler build (`tacplus_version`, `tacplus_sha`,
  `model/.hash.json` `binary_hash`) into `model/`, so a regeneration never matches the tracked files on a later commit
  or patch. The `gates` recipe now ignores those stamps; the lasting fix belongs in tacplus (stamp at build or run
  time, or from the proto inputs rather than `HEAD`), after which the `-I` filter in the justfile can go. No tacplus
  issue exists for it yet.
  Also (2026-10-29, [#8](https://github.com/tacplus-dev/kmaker/issues/8)): `model/registry.json` embeds tacplus's own
  docs (`USER.md`, …), so any patch of the floor's line that edits them (2.4.0 → 2.4.2 did) breaks the freshness check
  the same way. The same fix applies: keep compiler docs out of generated output, or reference them by version.
  Since 2026-10-30 ([#9](https://github.com/tacplus-dev/kmaker/issues/9), 2.5.1 → 2.5.3) the `gates` recipe also
  ignores the embedded doc lines (`-I '^    "[A-Z_]+\.md": +"'`); that filter can go with the same tacplus fix. It
  misses doc names outside `[A-Z_]` (a digit, say), which would fail the gates again until widened.
  And (2026-11-02, [#10](https://github.com/tacplus-dev/kmaker/issues/10)): 2.5.6 writes `model/registry.json` with one
  space after each `:` where 2.5.3 wrote two, so every line moved and the doc filter (then `":  "`) stopped matching;
  it now takes any number of spaces. The JSON formatting itself is compiler-dependent output too: generated JSON with
  a fixed, stable formatting (or a semantic JSON comparison in the gates) would keep such patches from failing them.
  Again (2026-11-03, [#11](https://github.com/tacplus-dev/kmaker/issues/11)): 2.5.8 is back to two spaces after each
  `:`, the third flip within the 2.5 line (2.5.3 two, 2.5.6 one, 2.5.8 two). All 28,493 lines of `model/registry.json`
  moved with no change under `git diff -w`. Formatting that changes this often is a further argument for the
  semantic comparison.
- **tacplus's build cache skips generation in a clean checkout** (2026-11-05, [#12](https://github.com/tacplus-dev/kmaker/issues/12)):
  `tacplus build` (2.5.11) reports "Build cache hit — inputs unchanged, skipping generation" when the committed
  `model/.hash.json` matches its inputs, without checking that the outputs exist; in a fresh clone the untracked
  part of model/ (`model/types/sys/…`, `model/chassis/…`) is then never written and `go build` fails. And a
  generation that does run rewrites `proto_sha` to the checkout's HEAD, so no committed tree is stable under it. The
  manager's fresh-checkout gates (after tacplus-dev/looper#79) hit both. `just gates` now removes the stamp to
  force generation and restores the tracked files after comparing them. The cache should verify its outputs (or
  offer a flag to bypass it) in tacplus-dev/tacplus; then the `rm` can go.
- **looper-trace does not see calls made through interface helpers** (2026-11-04, [#12](https://github.com/tacplus-dev/kmaker/issues/12)):
  `looper-trace check` reported `pass (0 handlers, 0 findings)` while `MintKernelCert` dropped its trace on both
  Kubernetes calls, because they go through the `TokenReviewer`/`PodGetter` interfaces, not a generated client, and the
  check does not follow helpers. It still reports 0 handlers now that the calls carry the trace. If the check treated
  a call that passes a method's `traceID` parameter, or a hand-written HTTP client's `http.NewRequest`, as a causal call,
  it would cover this handler too. That belongs in tacplus-dev/looper.
  The convention for non-tacplus HTTP calls (`traceparent`, else `X-Tacplus-Trace-Id`) is proposed for tacplus USER.md
  in [tacplus#317](https://github.com/tacplus-dev/tacplus/pull/317). Once it merges, the model/registry.json embedded
  docs pick it up with the next regeneration.
- **A security report in the build pipeline** (2026-11-06, nightly scan; [#4](https://github.com/tacplus-dev/kmaker/issues/4)):
  the nightly scan starts from the build pipeline's security report, but kmaker has none. README.md has no "Security
  checks" section, `.gitlab-ci.yml` has only build/package/manifest jobs (tag pipelines only), and `.github/workflows`
  holds only the planning-files check. So the 23 govulncheck advisories in #4 (9 reachable from kmaker's code) are
  found only by the nightly scan, and only after `model/` is generated: `govulncheck ./...` on a fresh checkout fails
  with `undefined: protoerrors.NewErrMissingInvalidData`, because the untracked part of model/ is missing. A
  workflow that runs `tacplus build` and then `govulncheck ./...` (plus an image scan of the alpine base) on every push,
  with a README section that says where to find its report, would catch these when they land (cf. tacplus-dev/roadmap
  ROADMAP.md Later, "Automatic security remediation", which builds on such scans; looper#49).
  Nearer (2026-11-07, [#5](https://github.com/tacplus-dev/kmaker/issues/5)): `just gates` generates model/ first, so a
  `govulncheck ./...` step there would work; it is left out only because it would fail every land until #4 is fixed.
  Once #4 lands, add it to the gates (pinned, not `@latest`, so the gates stay reproducible).
- **ROADMAP.md still names the tacplus floor 2.4** (2026-10-29, [#8](https://github.com/tacplus-dev/kmaker/issues/8)):
  the floor moved to 2.5 (tacplus.yaml, model/ regenerated by 2.5.1), but ROADMAP.md's "Now" ("tacplus floor 2.4") and
  its Later entry for #1 ("Declare the tacplus floor 2.4") still say 2.4. Loopers may not edit ROADMAP.md; Jody (via an
  agent) to change both to 2.5. Since 2026-11-06 ([#13](https://github.com/tacplus-dev/kmaker/issues/13)) the floor is
  2.6 (pinned to 2.6.1 in tacplus.lock), so both should now say 2.6.
  Jody asked for "Now" to name the floor in force ([#14](https://github.com/tacplus-dev/kmaker/issues/14)), but a
  looper's push that touches ROADMAP.md is refused. The line for an agent at his request: "Unreleased; tacplus floor
  2.6 (2.6.1 and its build image pinned in tacplus.lock)."; the Later entry for #1 is done and can go.
- **The looper's `tacplus` wrapper runs the lock's release but not its image digest** (2026-11-06,
  [#13](https://github.com/tacplus-dev/kmaker/issues/13)): tacplus.lock pins `build_image:
  ghcr.io/tacplus-dev/tacplus/build:2.6.1@sha256:…`, but the wrapper runs the pinned binary in `build:<line>` (the
  line's latest patch image), so the gates here and on land are not run in exactly the pinned image. Running
  `build_image` from the lock when there is one would make them deterministic as tacplus 2.6 intends (a
  tacplus-dev/looper change).
  Checked by hand on 2026-11-07 ([#14](https://github.com/tacplus-dev/kmaker/issues/14)): `just gates` passes inside
  the digest-pinned image itself (no docker in it, the image's own `tacplus` launcher, as tacplus-dev/build runs it).
- **The build's security checks fail on generated code until tacplus-dev/looper#108** (2026-11-07,
  [#14](https://github.com/tacplus-dev/kmaker/issues/14)): `looper-scan local` on main with the gates' generated
  `tacplus/` laid over it (as the security job does) fails the policy on 144 findings in compiler output (gosec G115
  ×120, G703 ×9, G404 ×6, G402 ×2, G123 ×2, G704 ×2; gitleaks generic-api-key ×3 in chassis test fixtures). The hub
  decided for blobstore#17: no per-service exceptions for generated code; services' SAST skips `tacplus/`
  (tacplus-dev/looper#108) and tacplus fixes the TLS findings at the source (tacplus-dev/tacplus#359). The one finding
  in hand-written files (the image ran as root, semgrep `missing-user-entrypoint`) is fixed in #14.
- **Old compressors in dsnet/compress's module graph** (2026-11-07, [#4](https://github.com/tacplus-dev/kmaker/issues/4)):
  after the #4 bumps `govulncheck ./...` (source and module mode) finds nothing, but `go list -m all` still lists
  `github.com/klauspost/compress v1.4.1` (GHSA-259r-337f-4rfw) and `github.com/ulikunitz/xz v0.5.6` (GO-2020-0016,
  GO-2025-3922), both required only by `github.com/dsnet/compress v0.0.1` for its own tests; `go mod why -m` says
  kmaker does not need them and they are not in the build. A scanner that reads the whole graph (OSV over
  `go list -m all`, as the nightly scan did) keeps flagging them. Its only importer is generated code
  (`tacplus/chassis/diskstore/compress.go`, `model/` before #3; `dsnet/compress/bzip2`), so the fix belongs in tacplus-dev/tacplus's
  generator: another bzip2 writer (dsnet/compress has had no release since 2018) would clear the graph.
- **Upstream product names in the compiler's own output** (2026-11-07, [#3](https://github.com/tacplus-dev/kmaker/issues/3)):
  this repository no longer names SeaBeam outside PROVENANCE.md and the dated records, and `just gates` now fails on any
  `seabeam|sonar/` match elsewhere. But the tracked `tacplus/registry.json` still carries upstream names that tacplus
  2.6.1 itself writes: the comments of its `sys/spec.proto` (`ScottyActivity`, `scottypersonal.ChatEvent`, as examples
  in `MethodCollection` and `RecordRead`) and its embedded `VERSIONING.md` ("until SeaBeam's launch"). The gates skip the
  embedded doc lines; the spec comments do not match the gates' pattern. tacplus-dev/tacplus could use neutral examples
  there (or keep its docs out of generated output, as "Reproducible generation" above asks); no tacplus issue exists yet.
- **ROADMAP.md's #3 entry can go** (2026-11-07, [#3](https://github.com/tacplus-dev/kmaker/issues/3)): the module is
  `github.com/tacplus-dev/kmaker`, the proto is vendored here and nothing is fetched from gitlab.com. ROADMAP.md is
  Jody's to change.

## Promoted / declined
