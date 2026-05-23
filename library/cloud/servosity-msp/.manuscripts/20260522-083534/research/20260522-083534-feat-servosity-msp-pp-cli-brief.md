# Servosity MSP CLI Brief

**Spec source:** `/Users/dstevens/Documents/Dev/cc-skills/servosity-msp-cli/spec/servosity-msp.openapi.json` (255→230 paths, partner-safe surface only).
**Auth:** Single `Token` header. Reseller-scoped — the API key implies a reseller; the CLI auto-resolves the reseller ID from `/current-user/` and never asks the user to type it.

## API Identity

- **Domain:** Co-managed backup & DR for MSPs. Three engines: classic (legacy), restic (modern), DR/DRaaS.
- **Users:** MSP technical staff and MSP owners running daily ops across their book of clients. Often working in Claude Code as the IDE/terminal. Their counterpart at Servosity (the assigned backup engineer) handles the bulk of monitoring — the MSP just needs visibility, fast triage, and reporting to the END client.
- **Data profile:** Reseller (me) → companies (my clients) → backups (3 engines) → backup-sets → backup-jobs. 230 partner-safe endpoints across 22 top-level resources. The fleet sits at ~10s–100s of companies per reseller, ~1000s of backups across the book.

## Reachability Risk

- **None.** Production API with bearer-token auth, no rate-limit/anti-bot horror stories in research. Direct HTTP from Go is fine.

## Top Workflows (derived from MSP persona research, not from servosity-pp-cli)

Research surfaced three load-bearing pain points for backup-managing MSPs in 2026:
1. **Context-switching between portals** — MSPs use 5–10 vendor dashboards; backup is one of them. Anything that pulls backup state into the terminal/Claude Code workflow removes that switch.
2. **Multi-tenant rollup** — "show me everything across all my clients in one view" is the headline ask. Most vendors claim multi-tenancy; few deliver a one-screen fleet view.
3. **QBR reporting** — every MSP runs quarterly business reviews per client. Backup is a load-bearing section (job success %, restore test outcomes, RTO/RPO, coverage). Most MSPs hand-build these decks each quarter.

The top workflows that fall out:

1. **Morning fleet sweep** — "what needs my attention right now across my book?" One command merges open issues + stale backups + in-flight DR events, ranked by company.
2. **Stale-backup follow-up list** — Friday/weekly view of clients I need to email about a stalled backup. Filterable by reseller scope, age, engine.
3. **Client QBR pack** — One command generates a per-client executive backup report: job success %, restore tests run this quarter, coverage map (which devices/engines), storage trend. Output Markdown or JSON; later layers can render to PDF.
4. **New-client onboarding** — Pull the agent install token for my reseller, list unprovisioned agents, push update-latest to standardize versions.
5. **Restore-queue oversight** — During an active DR event, watch every company's restore queue in one terminal.
6. **Bill reconciliation** — Pull my reseller's monthly bill (`/resellers/{id}/bill/`) and compare line-by-line against what I'm invoicing my clients. Surface drift.
7. **RMM/PSA bridge** — Convert Servosity issues to ConnectWise/Datto RMM/NinjaOne tickets. Not the core CLI's job, but the JSON output must be ticket-shaped enough that a small bridge script can do it.

## Table Stakes (must match)

These are competitor-baseline features for any backup MSP tool. Skipping them breaks adoption:

- Open-issue browse, ignore/archive/comment/reactivate
- Company/backup CRUD (read mostly; write for onboarding)
- Per-engine backup listing (classic, restic, DR)
- Stale-backup report (this is literally the most-clicked report in every MSP backup dashboard)
- Restore initiation visibility
- Agent install/restart/update (subset of agent-sessions — Damien's call, partner-useful only)
- Self-service billing data (bill, prices, subscriptions, contracts)

## Data Layer

A local SQLite mirror is load-bearing for the differentiator. Without it, the CLI is "a thin REST wrapper" — same shape as every competitor's PowerShell snippets. With it, the CLI answers questions the dashboard can't: trend, drift, cross-engine joins, offline filtering.

- **Primary entities to mirror:** companies, restic-backups, dr-backups, classic backups, backup-sets, issues, reports/stale-backup-sets snapshots, reseller bill/prices/subscriptions
- **Sync cursors:** modified-at on each resource where available; full re-pull for snapshot reports
- **FTS/search:** company name, backup hostname, issue title — let `find <text>` work offline
- **Snapshot history:** Save the JSON output of attention/stale-backups/issues each run, keyed by date. The `drift` command compares two snapshots. This is the foundation for "what got worse since yesterday."

## Source Priority

Single-source CLI (Servosity API only). No combo. No inversion risk.

## Codebase Intelligence

Not running DeepWiki — there's no public GitHub for the Servosity API, and we explicitly want to ignore `servosity-pp-cli` per the briefing.

## Product Thesis

- **Name:** `servosity-msp-cli`
- **Headline:** "The first MSP-fleet CLI for backup. Every Servosity REST endpoint as a typed command, plus a local mirror that lets you ask questions the dashboard can't — across your whole book of clients."
- **Why it should exist:** No competitor in the MSP backup category ships a fleet-wide CLI. N-able Cove has a per-machine ClientTool.exe; Acronis, Datto, Veeam — all dashboard-only. MSPs increasingly work in Claude Code / terminal. The CLI brings backup ops into the agentic workflow nobody else serves.

## Build Priorities

**Priority 0 (foundation):**
- Auth with reseller-scoped `Token` header
- Auto-resolve current reseller ID from `/current-user/` and cache it
- Local SQLite mirror for companies, all three backup engines, backup-sets, issues, reseller bill/prices/subscriptions
- `sync`, `search` (FTS5), `sql` (typed query), `doctor`, `current-user`

**Priority 1 (absorb — match every competitor and the API surface):**
- Every endpoint in the 230-path filtered spec as a typed command (companies, backups, restic-backups, dr-backups, backup-sets, agent-sessions partner subset, issues, reports, resellers self-ops, contracts, credentials, users, stats)
- `--json`, `--select`, `--csv`, `--dry-run`, typed exit codes on every command
- Agent-context discovery so Claude Code can introspect the surface

**Priority 2 (transcend — what nobody else has):**
- `attention` — one-screen morning sweep across the book (issues + stale + in-flight DR), per-company ranking
- `stale-backups` — offline-queryable slice of `/reports/stale-backup-sets/` by reseller/company/age/engine; `--refresh` to repull
- `drift` — diff two snapshots the CLI itself collected (attention, stale, issues), show what got worse and what recovered
- `qbr <company> --quarter 2026-Q1` — generate a Markdown executive backup report for a client's QBR (job success %, restore tests, coverage, storage trend)
- `triage` — list open issues with filters, batch ignore/archive/comment with `--dry-run` and typed exit codes
- `restore-queue list --watch` — watch every company's restore queue during active DR
- `bill --reconcile` — pull reseller bill and emit a JSON delta vs a CSV of what you're invoicing (input file flag)
- `unprovisioned` — list agents installed but not yet pulling backups, ranked by client

Each transcendence feature is impossible without the local mirror + multi-resource join. None exists in any competitor tool.
