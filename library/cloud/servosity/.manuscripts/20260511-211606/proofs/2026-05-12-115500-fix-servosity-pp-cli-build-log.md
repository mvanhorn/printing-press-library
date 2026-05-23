# Phase 3 Build Log — servosity-pp-cli

## What was built

### Store extension
- `internal/store/snapshots.go` — three snapshot tables (`attention_snapshots`, `stale_backup_sets_snapshots`, `dirty_repos_snapshots`) plus per-call `*_runs` tables, with idempotent `EnsureNovelTables()` and typed read/write helpers (`WriteAttentionSnapshot`, `WriteStaleSnapshot`, `LatestRunIDs`, `AttentionAt`, `StaleAt`, etc.).
- Avoids `COALESCE(...)` in PRIMARY KEY constraints (SQLite rejects expressions there); uses `NOT NULL DEFAULT ''` columns and plain composite PK instead.

### 10 transcendence commands (one file each, registered in root.go)
1. `attention.go` — composes 4 admin/issues endpoints, persists snapshot for drift comparison, supports `--reseller` filter and `--no-store`.
2. `drift.go` — diffs two snapshots (attention | stale) collected by the CLI itself, supports `--from`/`--to` human-time strings.
3. `stale.go` (`stale-backups` command) — pulls and parses the `/reports/stale-backup-sets/` CSV, handles the actual column names (`Partner`, `Last Backup Date`), computes `days_stale` from the Last Backup Date when the report omits it, infers engine from Backup Set / Destination text. Renamed from `stale` (generator already had a generic `stale` command).
4. `backup_facts.go` — cross-engine view (classic + restic + DR) over the synced backup tables.
5. `find.go` — cross-table FTS5 wrapper around the generator's `Search` with `--in resource1,resource2` filter and grouped output.
6. `restore_queue.go` — `restore-queue list [--watch]` composer over `/companies/{id}/restore-queues/`.
7. `company_show.go` — `company show <id>` composer over ~10 endpoints; gracefully degrades on per-section 404/403.
8. `triage.go` — batch issue triage with `--ignore`, `--archive`, `--reactivate`, `--comment`, `--ignore-until` (human time → `ignored_seconds`).
9. `clear.go` — daily Tier-One workflow: resolve names as company OR partner, batch-ignore issues until a human time.
10. `stale_issues.go` — daily Tier-One morning workflow: pull FMDB-by-current-user, classify by shipped rule table, plan auto-archive.

### Helpers
- `timewords.go` — human-time-string parser (`"6am tomorrow"`, `"yesterday"`, `"2h"`, `"+30m"`, RFC3339, etc.) with table-driven test (`timewords_test.go`).

### Production safety implemented end-to-end
- All 3 mutating commands (triage, clear, stale-issues) default to **PLAN mode**. `--confirm` is required to actually call mutation endpoints.
- All 3 also short-circuit when `flags.dryRun` (global root flag) is true OR `cliutil.IsVerifyEnv()` is true.
- All 7 read-only commands also short-circuit on `flags.dryRun` so verify probes never burn rate limit on the production API.
- The 10-command tree was vetted: no `--watch` daemon path is reached without explicit user opt-in; `restore-queue list --watch` short-circuits under `--dry-run`.
- Confirmed working against live API: `attention` returned 53 ranked rows from 4-endpoint composition; `company show 5914` returned 9 sections; `triage --company 5914` returned 2 real issue UUIDs with `will_mutate: false`; `clear "ZZZ_Fake" --until "6am tomorrow"` computed `ignore_seconds: 79508` with `will_mutate: false`; `stale-backups --refresh` pulled and parsed 849 CSV rows with `days_stale` correctly computed from the Last Backup Date.

## Intentionally deferred (out of scope for v1)
- **`agents stuck` / agent-restart orchestration** — the support team's `agent_restart_workflow.py` requires ScreenConnect integration (external SaaS). The Servosity-side actions (`/restic-backups/{id}/agent-service-stop/` etc.) ship as the absorbed typed commands; the cross-system orchestration belongs in `servosity-toe`.
- **Real-time agent telemetry via WebSocket** — `wss://api.servosity.com/ws/agent-interaction/` requires a Hydra OAuth2 token, not the API token. Worth a v2; v1 surfaces snapshot data via `/agent-sessions/{id}/`.
- **Specialist-category dispatch** (system-offline, low-disk, verification, ...) — these are `servosity-toe`-specific classifications driven by `match.conditions` rule files. The CLI surfaces raw issues; orchestrator-level routing belongs in the Python layer.

## Server-side / spec quirks discovered (and handled in code)
- `securityDefinitions` (Swagger 2.0) is invisible to kin-openapi → converted spec to OpenAPI 3 with `x-prefix: "Token"`, `x-auth-vars`, and `x-mcp` enrichment via `/tmp/convert_servosity_spec.py`. Generated CLI now sends `Authorization: Token <KEY>` correctly and reads `SERVOSITY_API_TOKEN` from env (live `doctor` confirms `auth_source: env:SERVOSITY_API_TOKEN`, `base_url: https://api.servosity.com/api/v1`).
- `/issues/?company=X` returns nothing without `state=ACTIVE` — every novel command that fetches issues passes `state: "ACTIVE"` automatically.
- `/issues/ignored/` is a **separate endpoint** from `/issues/`; novel commands only touch `/issues/?state=ACTIVE` (active triage scope).
- `/company-notes/?company=X` filter is broken — `company show` uses the **nested** `/companies/{id}/notes/` endpoint instead.
- `/companies/fully-managed-ng/` returns an unpaginated raw list; `stale-issues` parser handles both list and paginated envelope shapes.
- `PUT /issues/{id}/ignore/` body shape: `{ignored_seconds: N}` for timed, empty for permanent — `triage` and `clear` both compute `ignored_seconds` from `--ignore-until` / `--until` human time strings.
- `/reports/stale-backup-sets/` returns CSV with columns `Partner,Company,Backup,Backup Set,Destination,Last Backup Date` (NOT `Reseller`/`Last Complete`/`Days Stale`/`Engine`/`Owner` as I initially assumed). Parser corrected to handle actual column names; `days_stale` computed from `Last Backup Date`; engine inferred from Set/Account/Destination text.
- `/admin/attention/` returns empty body (not even `[]`) — composer handles this gracefully; `/admin/dirty-repos/` returns `[]` cleanly; `/admin/draas-in-progress/` returns `[{id, name}, ...]` (3 rows live).
- Long-running endpoints (e.g. `/reports/stale-backup-sets/`) need `--timeout 180s` on the global flag — documented in `stale-backups --help`.

## Generator limitations found (retro candidates)
- **`upsert_batch_test.go` failures** in 3 tests (start_backup, start_restore, restic_backups_tunnel) — NOT NULL constraint failures on a foreign-key column the typed upsert doesn't populate. Pre-existing in generator-emitted code, not introduced by Phase 3. Worth a Printing Press fix because it affects every Servosity-shaped Swagger 2 spec with foreign-key relationships derived from path params.
- **Swagger 2.0 ingestion needs manual conversion** — kin-openapi's `LoadFromData` parses Swagger 2.0 JSON enough to extract `paths` and emit Cobra commands, but silently drops `host`/`basePath`/`schemes`/`securityDefinitions`. Result: default `BaseURL: "https://api.example.com"` placeholder and no env var for auth. The Printing Press should detect `swagger: "2.0"` at load time and either auto-convert via `kin-openapi/openapi2conv` or refuse with a clear "convert to OpenAPI 3 first" message.
- **Generic `stale` command name collision** — the framework's PM-style `stale` (find unupdated items) collided with my domain-specific `stale` (backup-set freshness). Renamed mine to `stale-backups`. The Printing Press could namespace its framework-emitted commands under `pm` or detect domain conflicts.

## Known gaps documented for the README
- `attention --reseller X` filters but `/admin/dirty-repos/` and `/admin/attention/` don't accept a reseller param — the filter is applied client-side after the call.
- `attention` doesn't follow pagination on `/issues/?state=ACTIVE` past the first page; large fleets may need `--limit 0` and a follow-up. (TODO for v0.2.)
