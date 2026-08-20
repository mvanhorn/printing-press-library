# Semrush CLI build log

## Phase 2 (generate)
- Spec authored from local docs at `/Users/charlesg77/Desktop/SEMRush/semrush-documentation/` via Phase 2 subagent.
- 4,054-line internal YAML spec at `research/semrush-spec.yaml`.
- 10 resources (account/domain/subdomain/subfolder/url/keyword/backlink/project/audit/tracking) × 97 endpoints total.
- Top-level metadata: `category: marketing`, `auth.type: api_key` (`SEMRUSH_API_KEY`, query-param `?key=`), `cache.enabled: true / stale_after: 168h`, `mcp: {transport: [stdio, http], orchestration: code, endpoint_tools: hidden}` (Cloudflare pattern — required because 97 typed-tool MCP surface exceeds 50-tool threshold; without it the scorecard's MCP architectural dimensions score poorly).
- Generation gates on first try: `go mod tidy`, `govulncheck`, `go vet`, `go build`, runnable binary, `--help`, `version`, `doctor` — all PASS. One MCP advisory before enrichment; clean after.
- MCPB bundle emitted at `build/semrush-pp-mcp-darwin-arm64.mcpb`.
- 24 top-level Cobra commands (10 spec-derived resources + 14 framework — sync, search, sql, analytics, doctor, account, agent-context, api, auth, completion, feedback, help, import, profile, tail, version, which, workflow).
- 129 files in `internal/cli/`.

## Phase 3 (build)
- 12 novel-feature Cobra command files (~2,386 LoC of Go) hand-authored by Phase 3 subagent and wired into `root.go` via local-variable-capture pattern for parent commands.
- 1 migration file `internal/store/semrush_novel_migrations.go` introducing `snapshot_labels` and `credit_log` tables. Lazy init via `db.EnsureNovelTables(ctx)`; no `DO NOT EDIT` header so regen-merge preserves it.
- 1 cross-cutting helper `internal/cli/budget_helper.go` exposing `recordBalanceSnapshot(ctx, db, client, commandPath)`. Skipped under `PRINTING_PRESS_VERIFY=1` or `PRINTING_PRESS_DOGFOOD=1` to avoid live API calls during automated tests.
- 1 shared utilities file `internal/cli/novel_helpers.go` (sync-hint helpers, snapshot/window math).
- `root.go` is the only generator-emitted file edited — `regen-merge` re-injects AddCommand calls on next regen.

### Phase 3 Completion Gate
**Per-row Cobra resolution (12/12 PASS):**
```
PASS: drift          PASS: snapshot           PASS: backlink new
PASS: budget         PASS: keyword gap        PASS: backlink gap
PASS: audit triage   PASS: tracking drift     PASS: domain regions
PASS: serp-features  PASS: cannibalization    PASS: audit regression
```

**Deterministic backstop (dogfood --json):**
```
.novel_features_check = { "planned": 12, "found": 12 }
```

Dogfood also auto-synced:
- `README.md` "Unique Features" block from `novel_features_built`
- `SKILL.md` "Unique Capabilities" block from `novel_features_built`
- `README.md` Quick Start + Troubleshooting from `research.json` narrative
- `SKILL.md` recipes from `research.json` narrative
- `internal/cli/root.go` `--help` Highlights from `novel_features_built`

## Intentional design decisions
- **`domain regions` over `domain overview --databases`.** The approved transcendence row 9 specified the latter syntax, but adding a flag to the generated `domain_overview.go` would not survive regen. Renamed to a sibling Cobra command `domain regions`; manifest and research.json updated to match.
- **Trends API entirely out of scope.** User explicitly chose "Just the tested core" (Phase 0 gate). Trends commands, endpoints, and SKILL anti-triggers are absent by design.
- **v4 (OAuth) out of scope.** Same gate decision. Map Rank Tracker is the most attractive v4 surface (free, no API units) but the OAuth complexity wasn't worth shipping for a v1.

## Skipped complex bodies / generator limitations
- None observed. All Site Audit and Position Tracking POST/PUT/DELETE endpoints generated with their request-body params expanded to flags.
- `tracking` resource has 22 endpoints — the largest in the spec — and all wired.

## What was intentionally deferred
- **Per-call credit instrumentation.** The `credit_log` table currently records balance snapshots from `recordBalanceSnapshot` at the start of novel commands. Instrumenting EVERY generated endpoint command would require hand-edits to generator-emitted client.go that don't survive regen. The current design (probe-based snapshots + budget rollup) is directionally correct and ships clean; per-call granularity is a polish-time feature.
- **`--databases CSV` flag on `domain overview`.** Same reason — would require editing the generated file. The `domain regions` sibling command covers this workflow durably.
