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

## Promoted / declined
