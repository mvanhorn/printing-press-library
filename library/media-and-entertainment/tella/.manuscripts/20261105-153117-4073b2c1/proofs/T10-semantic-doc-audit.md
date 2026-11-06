# T10 — semantic descriptions and documentation audit

Base: `df4c73612e30af1ea8e1af98c2c07e634ff07110`. Current run: `20261105-153117-4073b2c1`.

## Corrected descriptions

Both `novel_features` and `novel_features_built` in the current run's `research.json` retain all 21 identities and change only these five descriptions:

- `sources upload-file` creates/uploads a source and returns its ID; `videos clips insert-file` provides upload-and-attach.
- `videos storyboard` exports editable timeline/state; `videos apply-storyboard --file PLAN.json --apply` applies the reviewed plan.
- `videos clips cut-words` plans/applies term-matched transcript-index cuts; `videos clips replace-word-ranges` provides replacement media.
- `videos clips undo-last-cuts` restores a saved snapshot, latest by default or selected with `--snapshot`; `videos clips restore-cuts` accepts inline cuts JSON. Divergence and exclusive-editing warnings remain intact.
- `webhooks tail` streams/snapshots the inbox; separate `webhooks replay` replays prior messages with valid HMAC headers.

Static `dogfood --research-dir RUN --overwrite-command-mirror` synchronized the primary README/SKILL capabilities, root Highlights, `which` and manifest's `novel_features`. The pinned runtime-walking path left the secondary references, MCP capability block, tools manifest and manifest's `novel_features_built` unchanged; minimal string edits brought those replicas into agreement. A second static dogfood run passed without reverting them. The rebuilt MCP `context` tool returned all 21 capabilities and the five corrected descriptions exactly. Existing command-specific Short/Long help did not make these false claims and was preserved.

README troubleshooting now documents a nonprinting credential presence check (never executed here) and a concrete videos listing command. SKILL Direct Use uses `account --agent` and `clips captions --help`; the legitimate `<command>` argument-hint metadata remains. Both unsupported exclusivity sentences now say “Workflows provided by this CLI.” Both final review notes distinguish scoped live evidence from incomplete generic coverage. The existing reprint patch's explanation records this intent; no duplicate patch was added.

## Verification

| Gate | Result | Evidence under `.pp-matrix/` |
| --- | --- | --- |
| Static dogfood | PASS, 21 planned / 21 found | `T10-static-dogfood-final.log` |
| Narrative strict/full examples | PASS, 3 commands | `T10-narrative.log` |
| Verify-skill | PASS, six checks / 126 recipes / no findings | `T10-verify-skill.log` |
| Go build / vet | PASS | `T10-build.log`, `T10-vet.log` |
| CLI / MCP binary builds | PASS | `T10-cli-build.log`, `T10-mcp-build.log` |
| Built runtime inventory | PASS, 288 CLI paths / 73 ordinary MCP tools / 29 read-only tools | `T10-runtime-proof.json`, `T10-runtime.log` |
| Public flags and schemas | Unchanged CLI operands/flags/annotations and MCP input schemas; terminal pagination cursor absent | `T10-runtime-proof.json` |
| Local help/discovery | PASS, all 21 novel helps plus five discovery/example helps | `T10-help.log` |
| Preservation and string-only audit | PASS | `T10-preservation.json`, `T10-preservation.log` |


## Scoped live evidence and limits

No generic live verify/dogfood, paid chat, unscoped sync, scorecard, API, vault, install, stage, commit, push or PR action ran. Full tests were intentionally skipped for string/docs-only edits. Local runtime surfaces were exercised; fresh live API behavior is **unverified** in T10.

## Binary identities

- Readonly Press: `bca064d7012a7d5954d5974b050ddd30aeefb6d8b1e457be3109fb54e806e672` (unchanged; never rebuilt/generated).
- Rebuilt CLI `<local-evidence>`: `d285bd7d3a980623b5941639cb05a4b8c0a020469a3319702178410b41ed1496`.
- Rebuilt MCP `<local-evidence>`: `bb7f4009b942e83cbeb780ea8a934a9cdc9063cfa392dfa79b83909d8971d014`.

Build/vet used the designated external GOCACHE and GOTMPDIR/TMPDIR. Final disposition and exact gate commands are in `<local-evidence>` and `<local-evidence>`.
