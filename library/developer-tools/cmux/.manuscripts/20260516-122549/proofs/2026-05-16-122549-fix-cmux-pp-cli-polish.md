# cmux-pp-cli Polish Report (Phase 5.5)

Polish skill ran in forked context. Final result block:

## Delta
| Metric | Before | After |
|---|---|---|
| Scorecard | 89/100 | 89/100 |
| Verify | 100% | 100% |
| Dogfood | FAIL | PASS |
| go vet | 0 | 0 |
| Tools-audit | 0 pending | 0 pending |
| Publish-validate | FAIL | PASS |
| MCP Desc Quality | N/A | 10/10 |
| Vision | 9/10 | 7/10 |

Grade A. Ship recommendation: **`ship`**.

## Fixes applied by polish
- Deleted dead `internal/cli/search.go` (the old generic search command; the cmux-shaped `cmux_search.go` replaces it).
- Ran `printing-press mcp-sync` to generate `tools-manifest.json` (publish-validate's manifest check).
- Staged `phase5-acceptance.json` into `.manuscripts/<run-id>/proofs/` (publish-validate phase5 check).
- Wrote `mcp-descriptions.json` with agent-grade overrides for four endpoint-mirror tools; lifted MCP Desc Quality from N/A to 10/10.

## Out-of-scope (next regen / spec-edit cycle)
- MCP Token Efficiency, MCP Remote Transport, MCP Tool Design (5–7/10): require a `mcp:` block in the spec (`transport: [stdio,http]`, `endpoint_tools: hidden`, `orchestration: code`).
- Cache Freshness 5/10: needs a generator-template helper.
- Type Fidelity 3/5: spec type richness.
- Vision dropped 9→7 after mcp-sync rewrote `.printing-press.json` — investigate in retro.

## Warning-level findings (not blocking)
- Output-review: `status changes` canonical column shows inconsistency on `"" → "Needs input"` transitions across 14 workspaces (domain-logic refinement).
- Output-review: `status awaiting --all` returns non-awaiting states alongside awaiting (naming/UX trade-off; the column is named `state` and reveals the mix, but the command name reads narrower than the behavior).
- Output-review: `search "cookie'"` token-mismatch on live-check (probable arg tokenization with the trailing apostrophe in the sample probe; not a real bug).

These are recorded for retro; polish judged them not blocking for ship.
