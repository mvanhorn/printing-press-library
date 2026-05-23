# Phase 3 Build Log — servosity-msp-pp-cli

## What was built

**Foundation (generator-emitted):** 441 generated typed Cobra commands across 230 endpoints, full local SQLite store, sync/search/sql/doctor/agent-context/MCP cobratree framework.

**Hand-built (10 transcendence features, this phase):**

| # | Feature | File(s) | Status |
|---|---|---|---|
| 1 | `attention` | `internal/cli/attention.go` | ✅ |
| 2 | `qbr` | `internal/cli/qbr.go`, `internal/qbrreport/qbrreport.go`, `internal/qbrreport/templates.go` | ✅ — MD/HTML/PDF formats, Chrome subprocess for PDF |
| 3 | `triage` | `internal/cli/triage.go` | ✅ |
| 4 | `restore-queue watch` | `internal/cli/restore_queue_watch.go` | ✅ |
| 5 | `backup-facts` | `internal/cli/backup_facts.go` | ✅ — UNION ALL across 3 engine tables |
| 6 | `drift` | `internal/cli/drift.go` | ✅ |
| 7 | `stale-backups` | `internal/cli/stale_backups.go` | ✅ |
| 8 | `unprovisioned` | `internal/cli/unprovisioned.go` | ✅ |
| 9 | `storage-trend` | `internal/cli/storage_trend.go` | ✅ — linear-regression forecast |
| 10 | `bill --reconcile` | `internal/cli/bill_reconcile.go` | ✅ — CSV reconciliation w/ integer cents |

**Shared helper:** `internal/snapshot/snapshot.go` — time-keyed JSON payload persistence used by `attention` and `drift`.

## Build verification

- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- Every new command responds to `--help` cleanly
- All 10 AddCommand registrations resolve

## Execution mode

10 parallel Opus subagents (one per feature), all spawned simultaneously. Each agent independently:
- Read 3-5 reference files (`issues_list.go`, `root.go`, `sync.go`, `helpers.go`)
- Wrote one to three files
- Ran `gofmt -l` to verify formatting
- Reported the AddCommand line back

Total wall-clock: ~6 minutes for the parallel wave (longest agent: `qbr` at 6 min; shortest: 2 min).

## Documented gotchas per feature

- **attention** — `/companies/{id}/restore-queues/` per-company iteration deferred to v0.2 (TODO comment in file); restore-queue count is always 0 in v1.
- **qbr** — Charts (storage trend lines, success-rate bars) deferred to v0.2. Storage trend renders as data table in HTML/PDF, ASCII sparkline + table in MD. Chrome detection chain: `chrome` → `google-chrome` → `chromium` → macOS app bundle path. Missing Chrome returns clean error.
- **triage** — Exit codes 4 and 5 collide with existing `authErr=4`/`apiErr=5`; built per-spec via `cliError{code:4|5}` directly. Doctrine collision flagged for retro.
- **restore-queue watch** — Prior snapshot is in-memory only (lost on restart). Adaptive limiter at 8 RPS floor.
- **backup-facts** — Field names probed via `COALESCE(json_extract(data, '$.alt1'), '$.alt2', ...)` since store schema isn't strongly typed for cross-engine fields. Will refine in dogfood if names miss.
- **drift** — Negative-delta sign formatting subtle; positive deltas get `+` prefix manually.
- **stale-backups** — `/reports/stale-backup-sets/` OpenAPI schema empty; envelope shape probed at runtime (`results`/`data`/`items`/bare array). Per-entry keys probed in snake_case AND camelCase.
- **unprovisioned** — Reseller ID extraction tries `reseller_id`, `resellerId`, `reseller.id` (nested), `reseller` scalar.
- **storage-trend** — Snapshots are stored under metric `storage-trend:<company_id>`. First-run state ("no history yet") returns successful, asks for periodic `--snapshot` invocation to build trend.
- **bill --reconcile** — Integer cents end-to-end. CSV header is case-insensitive and order-agnostic. Bill response unwraps `results`/`items`/`line_items`/`lines`/`bill` envelopes.

## Out of scope (deferred to v0.2 per absorb manifest)

- Charts in PDF/HTML for QBR (storage trend lines, success-rate bars)
- MSP-brand customization on QBR cover page
- Restore-queue persistence across `watch` restarts
- RMM/PSA bridge commands (still: downstream user-side script consuming `--json`)
- Real-time webhook receivers
- v1 PDF rendering requires local Chrome; document this clearly in README troubleshooting

## Known machine-level observations for retro

- 3/10 subagents (drift, stale-backups, storage-trend) ignored the explicit "DO NOT touch root.go" rule and added their own AddCommand line. Cause: the rule was buried under "REPORT TO ME AT THE END" formatting. Future runs should put the rule under a `## RULES` header that subagents can't miss.
- `triage`'s exit-code 4/5 collision with `authErr`/`apiErr` indicates the global exit-code palette needs documenting in CLAUDE.md / AGENTS.md so command authors don't accidentally collide.
