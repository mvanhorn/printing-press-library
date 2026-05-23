# Servosity CLI — Absorb Manifest

## Source survey
- **Web/MCP/CLI/SDK search:** No external tools, MCP servers, npm packages, PyPI packages, or competing CLIs touch the Servosity API. It is a private MSP backup product.
- **Existing internal artifact #1 (read-only baseline):** the user's own `cc-skills/servosity-api` shell wrapper — a generic `curl` shim with no domain features — and an auto-generated reference SKILL listing endpoints.
- **Existing internal artifact #2 (real support tooling — `~/Documents/Dev/servosity-toe`):** the Tier One Engineer toolkit. Python, ~6 support engineers use it daily. Composed of DH (Dashboard Helper), DTS (Diagnostic Tool), TH (Ticket Helper / Zammad), and DRaaS automation. Auth helper at `shared/helpers/servosity/_http.py` confirms the `Authorization: Token <KEY>` scheme, 30s timeout, and 3-attempt retry with (1s, 3s) backoff on transient errors. Endpoint helpers in `shared/helpers/servosity/{agents,companies,company_notes,dr_backups,draas,imagemanager}.py` are the ground-truth reference for what support actually calls.
- **DeepWiki / public source code:** none (no public repos for Servosity).
- **MCP source code:** none — the Compounding Teams agent has no Servosity surface today.

## Server-side quirks discovered (worth knowing for the build)
- Paths use **kebab-case** in the spec (`/current-user/`, `/restic-backups/`, `/dr-backups/`), not snake_case.
- `/issues/?company=X` returns nothing without `state=ACTIVE` — must always pass the state param.
- `/issues/ignored/` is a **separate endpoint** from `/issues/` (the regular endpoint never returns ignored issues regardless of state).
- `/company-notes/?company=X` ignores the company filter and returns ALL notes globally — must use the **nested** `/companies/{id}/notes/` endpoint instead.
- `/companies/fully-managed-ng/` returns an **unpaginated list** (no `count/next/previous/results` envelope) with sparse fields (`id`, `name`, `reseller__name`). Filterable via `?reseller__dedicated_support_staff__username=ME` for "show only the FMDB companies I own."
- `PUT /issues/{id}/ignore/` body shape: `{ignored_seconds: N}` for timed; empty body `{}` for permanent.
- `PUT /companies/{id}/backup-stores/` requires the **full record** (not a PATCH of one field) — you must read, mutate, and write back.
- Agent telemetry (real-time CPU / memory / time-drift / restic-running) lives at `wss://api.servosity.com/ws/agent-interaction/` and rejects the static API token — it requires a Hydra OAuth2 token. **Out of scope for v1**; the printed CLI surfaces snapshot telemetry from `/agent-sessions/{id}/` only.

## Absorbed (match or beat everything that exists)

The Printing Press generator absorbs all 328 method-endpoints into typed Cobra commands + MCP tools automatically. Each endpoint becomes a typed command at the path that mirrors the spec (e.g. `GET /admin/attention/` → `servosity-pp-cli admin attention`).

| # | Feature class | Best Source | Our Implementation | Added Value |
|---|---------------|-----------|-------------------|-------------|
| 1 | Every list endpoint (151 GETs) | spec / shell wrapper | Typed Cobra commands per path | `--json`, `--select`, `--csv`, `--compact`, `--page-size`, paginated automatically |
| 2 | Every write endpoint (177 POST/PUT/PATCH/DELETE) | spec / shell wrapper | Typed Cobra commands per path | `--dry-run`, typed exit codes, structured errors, `--json` request/response |
| 3 | Reseller / Company / Backup / DrBackup / ResticBackup / BackupSet / Issue / AgentSession / Contract reads | spec | Typed reads + local SQLite mirror | Offline querying after `sync`; `search` and `sql` over the store |
| 4 | All `/admin/*` rollups (attention, dirty-repos, draas-in-progress, ...) | spec | Typed admin subcommands | `--json` always, snapshot-to-store option |
| 5 | All `/reports/*` CSV downloads | spec / shell wrapper | Typed report subcommands | Stream CSV to stdout AND parse to SQLite snapshot for trending |
| 6 | All `/issues/*` triage actions | spec | Typed issue mutations | `--dry-run` everywhere; cursor pagination handled |
| 7 | Agent-session ops (restart spx/imagemanager, push notify settings, restic-interrupt) | spec | Typed agent-session subcommands | Confirmation gates on dangerous PUTs; `--dry-run` |
| 8 | Auth (`Authorization: Token <KEY>`) | spec / shell wrapper | `doctor`, `auth status`, env-var support | Validates against `/resellers/?page_size=1`; reads from `SERVOSITY_API_TOKEN` env or local config |
| 9 | Cursor + page-based pagination | spec | Generator's `paginatedGet` helper | Auto-follows `next` links and `cursor=` tokens; `--all` aggregates |
| 10 | MCP server | none (the bash wrapper has no MCP) | Cobra-tree mirror with Cloudflare pattern | `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, hidden raw endpoint tools — agents see thin search+execute pair instead of 328 tools |

## PRODUCTION SAFETY

**The Servosity API is single-tenant production.** Damien's token has admin scope. Therefore:
- Every novel mutating command (`triage`'s `--ignore/--archive/--reactivate/--comment`, `clear`, `stale-issues --auto-archive-known`) **defaults to `--dry-run`** and short-circuits when `cliutil.IsVerifyEnv()` returns true.
- A user must drop `--dry-run` AND pass an explicit `--confirm` (or per-command equivalent) to fire the mutation.
- Phase 5 live dogfood is **GET-only**. The Phase 5 test runner will be invoked with read-only safeguards — no PUT/POST/PATCH/DELETE will be executed against the live API.
- Admin broadcast endpoints (`/admin/notification/`, `/admin/notification-broadcast/`, `/admin/servosity-one-push-message/`, `/admin/servosity-one-push-update/`) are exposed (the generator emits typed commands) but **are never tested**, even with `--confirm`. They fan out to the whole user base.

## Transcendence (only possible with our approach)

These 10 commands ship as hand-written novel features beyond what the generator emits. The first 8 came from the adversarial subagent (all scored ≥7/10); the last 2 (`clear` and `stale-issues`) were added after examining the support team's daily Python workflows in `~/Documents/Dev/servosity-toe`. Each calls real endpoints or reads from the local store populated by `sync`.

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| 1 | Fleet attention rollup | `attention [--reseller X] [--json]` | 10/10 | Damien, Agent | One screen merges 4 endpoints (`/admin/attention/`, `/admin/dirty-repos/`, `/admin/draas-in-progress/`, `/issues/?status=open`) ranked per-company; persists each call to `attention_snapshots` so tomorrow's run can `drift` against today's |
| 2 | Stale-backup offline query | `stale [--days N] [--reseller X] [--engine X]` | 10/10 | Damien, Support Engineer | Reads from the `stale_backup_sets_snapshots` table populated by `sync stale` — slice/dice with no per-query API hit. Web UI ships only the raw CSV |
| 3 | Per-company snapshot | `company show <id> [--json]` | 9/10 | Damien, Support Engineer | Composes ~7 endpoints + the cross-engine `backup_facts` view into one screen; obsoletes a multi-tab UI flow |
| 4 | Terminal-speed issue triage | `triage [filters] [--ignore/--archive/--reactivate/--comment]` | 9/10 | Support Engineer | Lists `/issues/` with cursor pagination; batch flags call `/issues/{id}/{action}` per row with `--dry-run` and typed exit codes — web UI requires 5+ clicks per issue |
| 5 | Snapshot drift | `drift [--metric attention\|stale\|dirty-repos] [--from <ts>] [--to <ts>]` | 9/10 | Damien | SQLite query that diffs two `*_snapshots` rows the CLI itself collects; the web UI is point-in-time only |
| 6 | Cross-engine backup facts | `backup-facts [--company X] [--last-success-before <date>] [--engine X]` | 8/10 | Damien, Support Engineer, Agent | Queries the unified `backup_facts` view (engine + id + company_id + last_successful_at + last_status + size_bytes) populated from all three sync targets — the API has no cross-engine surface |
| 7 | Cross-table FTS find | `find "<query>" [--in companies,issues,backups]` | 7/10 | All three personas | SQLite FTS5 over companies (name, billing notes), issues (title, comments), backups (descriptive name, last error) indexed during sync; one query hits the whole fleet offline |
| 8 | Restore-queue oversight | `restore-queue list [--company X] [--watch]` | 7/10 | Support Engineer | Composes `/companies/{id}/restore-queues/` across companies the local store knows about; `--watch` is one-command poll with diff print, not a daemon — DRaaS-in-flight needs eyes-on |
| 9 | **Clear-company / clear-partner (NEW)** | `clear "<name>[, <name>...]" [--until "6am tomorrow"] [--dry-run]` | 10/10 | **Support (primary)** | Replaces today's daily `clear-company` Python workflow. Resolves each name as a company first, then a reseller; lists active issues per match; computes `ignored_seconds` from a human time string and batch-ignores. Multi-name comma-split. Defaults to `--dry-run` (PROD safety) |
| 10 | **Stale-issue cleanup (NEW)** | `stale-issues [--mine] [--cutoff "11pm yesterday"] [--auto-archive-known] [--dry-run]` | 10/10 | **Support (primary)** | Replaces today's morning `stale-issue-cleanup` Python workflow. Pulls FMDB filtered by current user, fetches active issues per company, classifies known-safe-to-archive patterns vs unknown by a shipped rule table, auto-archives safe + ignores non-dashboard noise, prints unknowns to stdout. Defaults to `--dry-run` |

**No stubs.** Every transcendence row above ships fully implemented. If a row turns out infeasible mid-build, I return to this gate with a revised manifest.

## Dropped from scope (with reasons)
- **`agents stuck` / agent-restart workflow:** the support team's `agent_restart_workflow.py` requires ScreenConnect integration (external SaaS) which v1 of this CLI cannot reach. The Servosity-side actions (`/restic-backups/{id}/agent-service-stop/` etc.) ship as the absorbed typed commands; the orchestration belongs in `servosity-toe`'s Python layer that already has ScreenConnect.
- **Real-time agent telemetry:** the WebSocket at `wss://api.servosity.com/ws/agent-interaction/` requires a Hydra OAuth2 token (not the API token). Worth a v2; v1 surfaces snapshot data via `/agent-sessions/{id}/` only.
- **Specialist-category dispatch (system-offline / low-disk / verification / ...):** these are `servosity-toe`-specific classifications that depend on `match.conditions` rule files. The CLI surfaces raw issues; orchestrator-level routing belongs in the Python layer that owns the cadence and rules.

## Source priority
- Single source. No combo CLI.

## Anti-patterns avoided
- No LLM-in-the-middle. `attention`, `triage`, `drift` are deterministic joins / diffs / rules.
- No client-side enum synthesis (uses the API's `audience` / `engine` / `status` enums verbatim).
- No fake aggregation endpoints — `attention` and `backup-facts` compose REAL endpoint calls or query SQLite populated by REAL sync.
- No "mirror but smaller" features — every transcendence row hits something the bash wrapper can't reach (local store, multi-endpoint join, history).
