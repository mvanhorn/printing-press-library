# Servosity MSP CLI — Absorb Manifest

## Ecosystem Reality

**Public tools that touch the Servosity API: zero.**
- No GitHub repos. No npm/PyPI wrappers. No MCP server. No published CLI. No community scripts.
- The only existing tool is the internal `servosity-pp-cli` (intentionally hidden from this run per Damien's direction — TOE-shaped, never validated).

**Category competitors (different APIs):** N-able Cove (ClientTool.exe — per-machine, Windows-only), Acronis Cyber Cloud, Datto BCDR, Veeam VSPC, ScalePad Backup Radar (aggregator).

Implication: there is no "match competitor parity" layer to absorb. The CLI's job is (a) match the full Servosity API surface as typed commands, then (b) transcend with fleet-view features no MSP backup vendor ships today.

---

## Absorbed (full API surface — Priority 1)

Each row = the entire endpoint family becomes typed Cobra commands. The printing-press generator emits these mechanically from the spec. Every command gets `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes for free.

| # | Resource family | Endpoints | Our value-add over a curl |
|---|---|---|---|
| 1 | `companies` | 67 | Local mirror, FTS, cross-engine joins, agent-context |
| 2 | `restic-backups` | 33 | Per-backup state in local store, history snapshots |
| 3 | `dr-backups` | 24 | DRaaS visibility, restore queue watching |
| 4 | `resellers/{id}/*` | 16 | Self-resolves reseller ID from `/current-user/` — never typed |
| 5 | `current-user` | 14 | API-token lifecycle, MFA backup codes |
| 6 | `backups` (classic) | 13 | Unified backup-facts view with modern engines |
| 7 | `agent-sessions` (partner subset) | 9 | install/restart/update/spx-activate/logs |
| 8 | `issues` | 9 | FTS + batch triage operations |
| 9 | `reports` | 9 | Snapshot each report's JSON locally for trend analysis |
| 10 | `contracts` | 5 | Sign flow, signature tracking |
| 11 | `credentials` | 4 | Rotate, version history |
| 12 | `users` (staff) | 4 | Reseller-staff management |
| 13 | `report-subscriptions` | 4 | Email subscription lifecycle |
| 14 | `stats` | 3 | Live + user stats with snapshot history |
| 15 | `backup-plans` | 2 | Plan inspection |
| 16 | `backup-sets` | 2 | Set CRUD across engines |
| 17 | `company-notes` | 2 | MSP-side note CRUD |
| 18 | `backup-search` | 1 | API-level search; CLI's `find` wraps + extends offline |
| Other | (backup-jobs, backup-job-report, backup-job-status, agent-login, components, download, screenshot, issue-comments) | ~10 | Long-tail; typed commands for completeness |

**Total absorbed: 230 endpoints → ~230 typed commands.**

Status of stub items in this list: **zero stubs.** Every absorbed endpoint ships as a real typed Cobra command with the full flag set. The generator handles the mechanics.

---

## Transcendence (impossible without our approach)

Each row is a feature **no MSP backup vendor ships today** because they all stop at the dashboard. These exist because of the local SQLite mirror + cross-resource joins + snapshot history. Each scored against the rubric (relevance 0–10).

| # | Feature | Command | Why only we can do this | Score |
|---|---|---|---|---|
| 1 | **Morning fleet sweep** | `attention` | Merges `/issues/` + `/reports/stale-backup-sets/` + `/companies/{id}/restore-queues/` into one ranked view. Requires three API calls + local join + ranking. No portal shows this. Persists per-call so tomorrow can drift against today. | 10 |
| 2 | **Stale-backup follow-up list** | `stale-backups` | Offline-queryable slice of `/reports/stale-backup-sets/` by reseller/company/age window/engine. Synced data; `--refresh` to repull. Friday's "who needs a follow-up?" sweep without burning N API calls. | 9 |
| 3 | **Drift since yesterday** | `drift --metric attention --from yesterday --to now` | Diffs two snapshots the CLI collected. Show what got worse and what recovered. Foundation for trend awareness. Trivially impossible without a local time-keyed store. | 10 |
| 4 | **Client QBR pack** | `qbr <company> --quarter 2026-Q1 --format pdf --out report.pdf` | Generates an executive backup report for one client's QBR: job success %, restore tests run, coverage map (devices × engines), storage trend, alerts trend. Pulls from local store + multiple reports endpoints. Three output formats: `--format md` (stdout), `--format html` (file), `--format pdf` (file, default). PDF rendering shells out to local Chrome (`chrome --headless --print-to-pdf`) with a clean MSP-brandable HTML template; falls back to MD with a clear error if Chrome missing. The single most-requested MSP backup feature per QBR research. | 10 |
| 5 | **Batch issue triage** | `triage --company 4421 --ignore 18,22,29 --comment "scheduled outage" --dry-run` | Multi-issue write in one invocation with `--dry-run` and typed exit codes. Web portal forces one-at-a-time. | 8 |
| 6 | **Watch restore queues** | `restore-queue list --watch --interval 30s` | Repolls every active company's restore queue and prints diffs. Pin one terminal during a DR event. Web portal forces tab-switching. | 8 |
| 7 | **Bill reconciliation** | `bill --reconcile invoiced.csv` | Pull `/resellers/{id}/bill/` and emit a JSON delta vs a CSV of what's been invoiced. Surfaces drift before month-end. | 8 |
| 8 | **Unprovisioned-agent watch** | `unprovisioned` | Joins `/resellers/{id}/agents/unprovisioned/` with companies to show: which agents are installed but not pulling backups, ranked by which client they belong to. Surfaces lost revenue. | 7 |
| 9 | **Storage trend per client** | `storage-trend <company> --weeks 12` | Snapshots each company's storage every sync; this command reads the time series and forecasts when they'll need more capacity. Revenue opportunity surfacing. | 7 |
| 10 | **Cross-engine backup-facts** | `backup-facts --company 4421` | Single unified view across classic + restic + DR for one client (engine, last successful at, last status, size). Joins three engine tables. Three API calls become one. | 8 |

**Total transcendence: 10 features, all scored ≥ 7/10.**

---

## Build-Phase Approach

- **Priority 0:** auth + reseller-ID auto-resolve + local SQLite mirror + sync + search + sql + doctor + current-user. Foundation for everything in Priority 2.
- **Priority 1:** mechanical — printing-press generator emits the 230 typed commands from the filtered spec.
- **Priority 2:** all 10 transcendence features above. These are hand-built.
- **Priority 3 (polish):** README/SKILL.md narrative, agent-context, MCP cobratree, scorecard polish.

---

## What we are NOT building (out of v1 scope)

- **RMM/PSA bridge commands.** ConnectWise/Datto/NinjaOne ticket creation is a downstream bridge script the user can build on top of `--json` output. Not the CLI's job to know about every PSA.
- **Branded QBR PDF with charts.** v1 PDF is data tables + clean prose + cover page. Charts (storage trend lines, success-rate bars) and MSP-brand customization land in v0.2 once partners use it and tell us which knobs matter.
- **Real-time webhook receivers.** The CLI is a synchronous client. Receiving Servosity webhooks for instant alerting is a separate daemon.
- **Cross-reseller views.** API key is reseller-scoped. Nothing to cross.
