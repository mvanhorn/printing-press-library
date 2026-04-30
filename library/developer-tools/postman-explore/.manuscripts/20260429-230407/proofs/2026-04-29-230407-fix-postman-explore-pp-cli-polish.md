# postman-explore-pp-cli — Polish Pass

## Delta

|                  | Before   | After    | Delta              |
|------------------|----------|----------|--------------------|
| Scorecard        | 82/100   | 84/100   | +2                 |
| Verify pass rate | 95%      | 95%      | unchanged          |
| Dogfood          | PASS     | PASS     | unchanged          |
| Verify-skill     | PASS     | PASS     | unchanged          |
| Workflow-verify  | PASS     | PASS     | unchanged          |
| Tools-audit      | 0 pending | 0 pending | unchanged         |

## Fixes applied (Phase 5 inline + this pass)

- `buildProxyPath` joins additional query params with `&` when the path already contains `?`. Resolves the silent sync failure where `/v1/api/networkentity?entityType=collection` got pagination appended as a second `?…` and the proxy parsed it as a malformed URL.
- Novel commands (`top`, `velocity`, `drift`, `publishers top`, `category landscape`, `similar`) now query the `resources` table filtered to networkentity sub-types. Sync routes typed entities (`collection`, `workspace`, `api`, `flow`) through the generic `db.Upsert` path; querying the typed `networkentity` table found nothing.
- `--category` accepts slugs (`developer-productivity`, `payments-apis`) or numeric IDs. Resolution lives in `internal/cli/category_resolve.go`; numeric input short-circuits with no network call.
- The promoted `search-all` shortcut was emitting `c.Get` against a POST-only endpoint and 400'd. Marked it `Hidden: true` and `mcp:hidden: "true"`. Users get the working `canonical` for live search; the typed `search-all search_all` subcommand still exists for power users.
- README: removed Retryable / Confirmable / Piped-input bullets that don't apply to a read-only API. Replaced with a single Read-only bullet.
- README Quick Start: `postman-explore-pp-cli stats` (which doesn't exist) → `postman-explore-pp-cli networkentity get-network-entity-counts`.
- README Troubleshooting: reinstall path corrected to canonical printing-press-library module path; "Run the `list` command" replaced with named list commands (`networkentity list-network-entities`, `category list-categories`).
- SKILL.md and README Discovery Signals: stale "Candidate command ideas: browse; get; stats; …" replaced with actual emitted command surface.
- Phase 4.85 output review (run during shipcheck): PASS — no plausibility findings on `canonical`, `drift`, `category landscape` samples.

## Skipped findings

- `mcp_token_efficiency` 4/10 — Postman entity payloads include rich descriptions (5–10 KB each) and metric arrays. The `--compact` and `--select` paths already exist; the score reflects upstream payload size, not a fixable CLI defect.
- `mcp_remote_transport` 5/10 — stdio only. Remote HTTP/SSE transport requires generator-level emission via the spec's `mcp:` block, not in scope for this regeneration.
- `mcp_tool_design` 5/10 — endpoint-mirror tools only. Intent tools and code-orchestration are generator-level capabilities; would require respec'ing.
- `cache_freshness` 5/10 — opt-in machine-owned freshness contract not enabled. Sync is on-demand, which is appropriate for this discovery surface.
- `type_fidelity` 3/5 — modest gap with no specific defect surfaced; spec models are reasonable.

## Remaining issues (retro candidates for the Printing Press)

- **SQLITE_BUSY race on concurrent fresh-DB migration.** When live-check or any parallel-command harness opens a fresh DB simultaneously across processes, both race the PRAGMA user_version stamp. Affects test parallelism, not real users. Fix: migration should use immediate-mode transactions or a file-level mutex.
- **Sync dispatcher routes typed sub-resources to generic upsert.** Resources `collection`, `workspace`, `api`, `flow` write to the generic `resources` table instead of the typed `networkentity` table. Novel-feature queries against the typed table found nothing. Fix: generator's sync dispatcher should route these to `UpsertNetworkentity`.
- **Promoted-shortcut emits GET for POST-only endpoints.** The generator's promotion logic doesn't detect HTTP verb on the underlying endpoint. Fix: detect verb and emit `c.Post` or skip promotion.

## Ship recommendation

**ship.** All gates met:
- shipcheck PASS (5/5 legs)
- verify-skill exit 0
- workflow-verify not workflow-fail
- tools-audit zero pending findings
- behavioral verification of all 8 novel features against live API confirmed plausible output
