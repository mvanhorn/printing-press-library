# Servosity API CLI Brief

## API Identity
- **Domain:** Backup and disaster-recovery for MSPs and end-customers (BDR-as-a-service).
- **Owner:** Servosity (Damien Stevens, CEO). The user is the API owner.
- **Surface:** 255 paths / 328 method-endpoints / 33 resource tags. Three coexisting backup engines (classic `backups`, `restic-backups`, `dr-backups`) plus billing, contracts, agent management, issues triage, and admin rollups.
- **Users (today):** Damien + the small Servosity team (~7 ppl). Resellers and end-company users hit a web UI; programmatic access is essentially internal so far.
- **Data profile:** Hierarchical fleet — Reseller → Company → Backup (any of three engines) → BackupSet → BackupJob. Issues attach at any level (audience filter). Agent sessions cross-link.
- **Existing tooling:** the cc-skills `servosity-api` bash wrapper (curl shim) and the auto-generated reference SKILL. No proper CLI exists; this is the first one.

## Reachability Risk
- **None.** Live `GET /resellers/` returned 200 (127 resellers visible) with a 40-char admin Token. `Authorization: Token <KEY>` scheme verified. No bot-protection signals.

## Top Workflows
1. **Morning fleet health rollup** — one command answers "what needs my attention right now?" by joining `/admin/attention/`, `/admin/dirty-repos/`, `/admin/draas-in-progress/`, and `/issues/` (open) with per-company breakdown.
2. **Stale-backup hunt across the fleet** — `/reports/stale-backup-sets/` is a one-shot CSV today. The CLI syncs to SQLite and lets you slice by reseller, company, age window, backup engine, or owner.
3. **Issue triage at terminal speed** — list, filter (audience/company/reseller/backup), comment, ignore, archive, reactivate — all without context-switching to the web UI. Batch operations across selections.
4. **Per-company snapshot** — one command pulls a company's backups (all three engines), last successful job per set, open issues, agent versions, contracts, addresses — into a single human or `--json` view.
5. **Restic operational ops** — dirty repos, agent service restart, encryption key version visibility, agent-session log push.
6. **Reports to stdout** — pipe `/reports/account/`, `/reports/clients/`, `/reports/usage/`, `/reports/classic-usage/` through `jq`/`csvlook`/Excel without opening email.

## Table Stakes (vs the bash wrapper + web UI)
- Every endpoint reachable as a typed command.
- `--json`, `--select`, `--csv`, `--compact`, `--dry-run` everywhere.
- Local SQLite store mirroring the fleet so queries are offline and composable.
- Pagination handled (DRF default: `count`/`next`/`previous`/`results` + `num_pages`; `cursor` on issues).
- Honest error envelopes (`Token` rejected → 401, scope-limited reads → 403, missing entity → 404).
- Exit codes typed (`0/2/3/4/5/7/10`).
- MCP server so Claude/Anthropic agents (Damien's Compounding Teams stack) can drive it.

## Data Layer
- **Primary entities (sync-targets):** `resellers`, `companies`, `backups`, `restic_backups`, `dr_backups`, `backup_sets`, `issues`, `agent_sessions`, `contracts`, `users`, `reports_cache`.
- **Sync cursor:** Resource-by-resource — list endpoints with page/page_size for most, `cursor` for `/issues/`. Snapshot-only for `/admin/*` and `/stats/*` (no per-record cursor; persist with `synced_at`).
- **Cross-engine table:** unified `backup_facts` view (engine + id + company_id + last_successful_at + last_status + size_bytes) so transcendence queries don't need to know which engine.
- **Snapshots over time:** `stale_backup_sets_snapshots`, `attention_snapshots`, `stats_snapshots`. The web UI gives you the present; the CLI gives you trends. Drift = first-class.
- **FTS5:** companies (name, billing notes), issues (title/comment), backups (descriptive name + last error message). Cross-table FTS for "find anything that says 'image manager' across the fleet."

## Codebase Intelligence
- **Spec format:** Swagger 2.0 (not OpenAPI 3). Generator must consume `swagger: "2.0"`. Response bodies are sparse in the spec (`responses: { "200": { description: "" } }` with no schema), so the printed CLI surfaces will be parameter-typed but response-untyped — handlers will return raw `map[string]any` / pass-through JSON.
- **Auth:** `securityDefinitions.Token` (apiKey, header `Authorization`). **Wire format requires the literal `Token ` prefix** (Django REST framework convention) — the spec does NOT encode this prefix. The generator's default api_key emit would send `Authorization: <KEY>` which would fail. **Pre-generation enrichment must add the `Token ` prefix.**
- **Pagination:** standard DRF (`count`, `next`, `previous`, `results`, sometimes `num_pages`). Issues use `cursor`. The generator's `paginatedGet` should infer correctly from query-param shape.
- **Server-side rollups already exist:** `/admin/attention/`, `/admin/dirty-repos/`, `/admin/draas-in-progress/`, `/companies/summary/`, `/companies/summary-ng/`, `/companies/fully-managed/`, `/companies/fully-managed-ng/`, `/reports/stale-backup-sets/`. These are not transcendence to wrap directly — transcendence comes from joining/diffing them.
- **Engine duality (`-ng` paths):** the API exposes both legacy and "next gen" variants (e.g. `summary` vs `summary-ng`). Both stay until migration is done; the CLI exposes both as siblings, no opinion on which is right.

## User Vision
> "../cc-skills has servosity-api which uses auth somehow, you can discover to test"

Reading between the lines plus Damien's operating doc: this CLI is for **him** — the Loop Architect — to operate Servosity from the terminal/agent surface he lives in, not the web UI. The headline is "fleet observability + triage from agents," not "wrap every endpoint." The CLI is also Compounding Teams dogfood: Damien's team should be able to ask Claude "what needs attention" and get back a real answer assembled from real data.

## Source Priority
- Single source: the official Swagger 2.0 spec at `https://api.servosity.com/docs/?format=openapi`.
- No combo CLI; no priority inversion to worry about.

## Product Thesis
- **Name:** `servosity-pp-cli` (binary), `servosity` (slug, library directory).
- **One-line:** Every Servosity endpoint, plus a local fleet mirror, cross-engine rollups, and offline drift detection that the web UI has never offered.
- **Why it should exist:**
  - Damien's daily question is "what's open across the fleet?" — that's exactly what the CLI answers in one call.
  - The web UI gives "what's true now" but no history. Local SQLite snapshots turn `stale-backup-sets` and `attention` into trend lines.
  - Compounding Teams: any agent (Claude, ChatGPT, the orchestrator skills already in this stack) can drive a typed MCP server with proper auth — replacing the bash wrapper that lives in cc-skills today.
  - The bash wrapper is read-only-friendly; mutating endpoints (issue triage, agent restarts) want `--dry-run`, `--json`, structured errors. The wrapper doesn't give you any of that.
- **Audience-grain check:** This sits *with the grain* for Damien — terminal-first, agent-driven, surfaces-open-loops. Anti-grain would be a web dashboard (already exists), or yet another auth design (the existing `Token` scheme stays).

## Build Priorities
1. **Spec ingestion + auth enrichment** — emit Swagger 2.0 → typed Cobra tree, `Authorization: Token <KEY>` (with the prefix), `SERVOSITY_API_TOKEN` env var, `doctor` validates against `/resellers/?page_size=1`.
2. **Data-layer foundation** — SQLite store for resellers, companies, all three backup-engine tables, backup_sets, issues, agent_sessions, contracts. `sync` covers each resource with proper pagination. Cross-engine `backup_facts` view.
3. **Absorb every endpoint** — typed commands for all 328 method-endpoints (the generator does this). Hide deeply-nested mutations behind `--confirm`.
4. **MCP enrichment** — Cloudflare pattern (>50 tools threshold): `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden`. Avoid drowning agents in 328 raw tool surfaces.
5. **Transcendence (Phase 1.5c.5 subagent will refine)** — drafted candidates:
   - `attention` — fleet rollup (admin attention + open issues + dirty repos + draas-in-progress) with per-reseller drill-down
   - `stale --days N --reseller X` — offline query against synced stale-backup-sets snapshots, grouped however
   - `company show <id>` — one-screen per-company snapshot across all three engines + issues + contracts
   - `triage` — interactive issue triage from the terminal with batch ignore/archive/comment
   - `drift` — diff two attention/stale snapshots ("what changed since yesterday morning?")
   - `agents stale` — agent sessions that haven't checked in in N hours
   - `find` — FTS5 across companies, issues, backups (cross-table)
   - `doctor --deep` — auth + base reachability + each pagination contract + a write canary on a known-test entity (opt-in, with `--confirm`)
