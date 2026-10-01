# Independent findings and verification

Exactly one independent fresh-context gpt-6.1-sol reviewer, xhigh, fork_turns none; reused throughout, no edits or additional workers.

| Severity | Initial location | Reproduction | Fix / verification |
|---|---|---|---|
| P1 | MCP cache destination surface | inventory_refresh tool accepted caller cache-dir and overwrote existing inventory.json | Dedicated MCP root omits cache-dir from schema/allowlist; independent sentinel attempt rejected |
| P2 | internal/ecbo/client.go (initial line 218) | 256 unrelated JSON files in cache-dir; oldest deleted on refresh | Owned ecbo-response SHA filename namespace only; independent 256-file sentinels preserved |
| P2 | internal/ecbo/domain.go (initial line 128) | Repeated validation 503 became successful unknown offer | Typed upstream failure returned; deterministic and independent source-failure checks |
| P2 | search / validation response parsers | Missing search name/total accepted; arbitrary 422 JSON became rejection | Essential shape and recognized structured domain errors validated |
| P2 | inventory parser | {} silently empty; results:[1] panicked under projection | Snapshot data/result identity/timestamp validation, typed exit 10 |
| P2 | ecbo_commands.go projection loop | Empty inventory with unknown select field exited 0 | Public list schema validated independently of result count; exit 2 |
| P2 | SaveInventory filesystem boundary | inventory.json directory caused rename failure / exit 5 | Typed cache error / exit 10 |
| P2 | explicit task home normalization | Padded or whitespace-only --home diverged from framework normalization | Normalize before branch; ordinary/padded/empty cases regression tested; same-reviewer verification cleared |

Initial line references describe the pre-fix additive review snapshot in review.diff. Current source differs after fixes. Independent live reads matched raw provider price 1300 JPY and valid=true, source-rejected outside-hours response, Tokyo source IDs/coordinates, and 3-result inventory display with 50 saved records/offline reads.

The Press runner installed globally predates local-write fixture execution support. An existing corrected source checkout is compiled into this run's isolated tool-bin only; no shared source/binary/configuration changed. Local fixture files are cache state; provider data in live proofs remains live. The full marker must be produced by that tool, never hand-edited.

Final generated-preview correction: original raw endpoint dry-run mixed JSON/plaintext. A hook now emits one explicit request-preview plan; absolute source URLs preserved. Same reviewer verified all four exact URLs, planned metadata and oversized-stdin failure. Final reviewer reports no remaining actionable findings.
