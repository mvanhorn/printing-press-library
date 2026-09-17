# Conduyt CRM CLI Brief (reprint 2026-09-16, timing experiment #91)

Reuses the 2026-07-11 brief (manuscripts/conduyt-crm/20260711-202005) with the machine-delta re-validation folded in (printing-press 4.28.0 → 4.32.1; skill 2.0.0 → 3.0.0).

## API Identity
- Domain: conduyt.app — first-party AI-native CRM (Next.js 16 / Prisma 7 / Neon / Vercel). Live OpenAPI 3.1 at https://conduyt.app/openapi.yaml: ops 826 paths 572 domains 95 (July print: 508 paths / 726 ops).
- Users: (1) Paul — owner/admin (BDA ops: imports, drips, delivery forensics, dialer + reports); (2) headless AI agents driving contacts/deals/automations/reports via Bearer keys (the #1 persona); (3) ops scripts (n8n, cron).
- Data profile: contacts (200k+ at BDA), deals, pipelines, automations + execution logs, drip ledgers, import jobs, messages, invoices, NEW since July: reporting (lead activity, dashboards, funnel, appointments, calls, agent performance, attribution, scheduled deliveries), Smart Dialing (queues, dispositions, agent statuses, smart views), batch operations by filter, WhatsApp / voice drop / FromK.ai channels, local presence, contact identifiers, node-level operating hours on workflow steps (live 2026-09-16 10:04 ET).

## Reachability Risk
- None. First-party API; /api/v1/health → 200 at 14:16Z; Bearer key on hand (read-only smoke). Rate limits per route (30–120 per 15 min typical; public booking slots 120/15 min per IP).
- Machine-delta re-validation (4.28 → 4.32): transport unchanged (plain HTTPS, no clearance needed); auth unchanged (BearerAuth); scoring rubric now has MCP dims (remote transport / tool design / surface strategy) — the July Cloudflare-pattern MCP choice still scores as intended; discovery gates not applicable (canonical --spec).

## Top Workflows
1. Reports as commands: pull lead activity / speed-to-lead / funnel / appointments / calls / agent performance / attribution by range, as JSON or CSV, for agents and cron (NEW; the biggest surface added since July).
2. Bulk lead import with pre-flight safety: preflight → import (verifyLineType/reEnrollMode) → watch job → delivery outcomes (carried from July; still the Kloudi workflow).
3. Dialer ops: queue state, dispositions → lead status, agent statuses, smart views (NEW).
4. Contact/deal CRUD + search + bulk ops (batch by filter, select-all-matching) for agents (table stakes).
5. Automation ops: validate/import/dry-run/publish, step logs, operating hours on steps (NEW field).

## Table Stakes
- Full endpoint mirror of the 826-operation spec. Sync/SQL/FTS local store. --json/--select/--dry-run/typed exits. Doctor. Agent context. MCP with the Cloudflare pattern (stdio+http, code orchestration, hidden endpoint tools).

## Data Layer
- Primary entities: contacts, deals, companies, tasks, automations, import_jobs, messages, appointments, calls.
- Sync cursor: updatedAt per resource. FTS: contacts/deals/notes.

## User Vision
- Paul 2026-09-16: keep BOTH CLIs (this public-library print = marketing listing on printingpress.dev; ptaramona/conduyt-crm-cli = the current one). This run measures their update pipeline: open→merge time decides how often the listing is refreshed. Bias novel features toward reports-as-commands and dialer/queue workflows for AI agents; re-score the July novel features against current personas (keep / reframe / drop with reasons).

## Product Thesis
- Name: conduyt-crm-pp-cli ("Conduyt CRM")
- Why: the official terminal + MCP surface of a CRM whose differentiator IS AI-accessibility; agents get offline search, bounded output, and the same reports / verification / forensics tools the app ships.

## Build Priorities
1. Full mirror of the 826-op spec (absorbs July coverage + the ~100 new operations).
2. First-class report commands (range/group-by/format) and dialer queue/disposition commands with agent-native output.
3. July novel features re-scored: send-check, imports blame/watch, verify estimate, drips audit, analytics, tail.
4. Local store + sync/search/SQL (framework), MCP Cloudflare pattern.
