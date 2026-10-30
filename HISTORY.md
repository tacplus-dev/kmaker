# History: kmaker

Append-only: add entries at the end, never edit or remove one (CI enforces it). One line each:
`- <YYYY-MM-DD> <who: looper|agent|jody> <kind: issue|decision|release|deploy> <one line> (<links>)`

- 2026-10-28 looper issue Gates can never pass: the model/ freshness check compares build stamps (proto_sha = git HEAD, compiler patch) (#7, 75c0ac6)
- 2026-10-29 looper issue Pins: tacplus floor 2.4 is behind the latest released line 2.5 (and model/ is stale even on 2.4.2) (#8, 310c82e)
- 2026-10-30 looper issue Gates fail on main with tacplus 2.5.3: model/registry.json embeds the compiler's docs, which changed since 2.5.1 (#9, ad60280)
