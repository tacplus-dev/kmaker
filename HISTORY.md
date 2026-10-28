# History: kmaker

Append-only: add entries at the end, never edit or remove one (CI enforces it). One line each:
`- <YYYY-MM-DD> <who: looper|agent|jody> <kind: issue|decision|release|deploy> <one line> (<links>)`

- 2026-10-28 looper issue Gates can never pass: the model/ freshness check compares build stamps (proto_sha = git HEAD, compiler patch) (#7, 75c0ac6)
