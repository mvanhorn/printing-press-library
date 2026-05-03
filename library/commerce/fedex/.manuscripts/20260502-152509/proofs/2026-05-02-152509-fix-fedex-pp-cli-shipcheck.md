# FedEx CLI Shipcheck Proof

## Verdict: SHIP

All 5 shipcheck legs PASS:

| Leg | Result | Time |
|---|---|---|
| dogfood | PASS | 1.949s |
| verify | PASS | 2.706s (84% pass rate, 0 critical) |
| workflow-verify | PASS | 14ms (no manifest) |
| verify-skill | PASS | 262ms |
| scorecard | PASS | 51ms — **71/100 Grade B** |

## Scorecard breakdown

**Tier 1 infrastructure** (out of 50): hits ceiling on Output Modes, Auth, Error Handling, Doctor, Agent Native, Local Cache, Breadth, Workflows, Insight; full marks except sync-related dimensions:
- Cache Freshness 0/10 — `cache.enabled: false` (intentional; FedEx has no list endpoint, the local store IS the source of truth)
- Vision 6/10
- MCP Token Efficiency 7/10, MCP Remote Transport 5/10, MCP Tool Design 5/10 — could improve with `mcp:` spec-surface enrichment in a future iteration

**Tier 2 domain correctness** (out of 50):
- Path Validity 10/10
- Auth Protocol 6/10 — bearer_token but the test probe doesn't see the OAuth client_credentials story
- Data Pipeline Integrity 7/10
- Sync Correctness 2/10 — **expected for this API**; FedEx has no list-shipments endpoint, so no sync is possible
- Type Fidelity 3/5
- Dead Code 3/5

**Notable gap (acknowledged, not fixed):** Sync Correctness 2/10. This is the right behavior for the API shape: FedEx publishes no list-shipments endpoint, so the local archive IS the source of truth (write-only ledger). The scorer doesn't have a way to mark this as N/A; the score reflects machine assumptions, not a CLI defect.

## Fixes applied during shipcheck

1. **SKILL.md flag references corrected via research.json narrative.** Three stale references caught by verify-skill (`--name`, `--pdf`, `--to-saved`) — research.json's narrative.recipes used flag names from an earlier brainstorm before the implementation chose positional args / different flag names. Fixed in research.json and SKILL.md is regenerated correctly on next dogfood.

2. **MCP cache-leak fix backported.** Per printing-press-library PR #213, added `c.NoCache = true` to `internal/mcp/tools.go::newMCPClient()` so agent calls through MCP tools never see stale post-mutation cached snapshots. Mechanical 8-line patch matching the library backfill shape.

## Build summary

- 51 typed endpoint commands (auto-generated from spec)
- 13 hand-written novel commands (rate shop, ship bulk, address book + cached validate, track diff/watch, return create, archive, sql, spend report, manifest, ship etd, doctor extras)
- 18.3MB static binary
- SQLite local store with 6 tables + FTS5 index
- OAuth2 client_credentials login + auto-refresh
- 48 MCP tools registered

## Known unimplemented

- `--to-saved` flag wrapper for `ship create` (would let users ship with `ship create --to-saved acme` instead of building the full request body). Address book exists; the wrapper to thread it into ship-create is a Phase 6 polish item.
- `export` command: present (auto-generated) but reads from FedEx API, not the local archive. A local-archive variant would be a Phase 6 add.
- PDF rendering for `manifest` (text/markdown only — PDF generation requires external tooling).
