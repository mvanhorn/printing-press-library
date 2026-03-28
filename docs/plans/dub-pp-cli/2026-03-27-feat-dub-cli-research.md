---
title: "Research: Dub CLI"
type: feat
status: active
date: 2026-03-27
phase: "1"
api: "dub"
---

# Research: Dub CLI

## Spec Discovery
- Official OpenAPI spec: https://api.dub.co (served directly at root URL)
- Source: Speakeasy SDK workflow (dubinc/dub-go/.speakeasy/workflow.yaml)
- Format: JSON, OpenAPI 3.0.3
- Endpoint count: 48 endpoints across 14 resources
- Spec size: 357KB

## Competitors (Deep Analysis)

### sujjeee/dubco (24 stars) — THE ONLY DUB CLI
- Repo: https://github.com/sujjeee/dubco
- Language: TypeScript (96.4%)
- Commands: 3 (login, config, link)
- Last commit: August 2024 (~7 months ago)
- Open issues: 1
- Contributors: Failed to load (likely 1-2)
- Maintained: **No** — no commits in 7 months
- Notable features: Interactive link creation with nanoid, custom short keys
- Weaknesses:
  - Only creates links. No list, get, update, delete, search.
  - No analytics, domains, tags, folders, customers, partners.
  - No --json output. No agent-friendly features.
  - No bulk operations.
  - Interactive-only (no flag-based creation).
  - No export/import capability.

### Dub Official CLI (dubinc/dub#506) — CLOSED
- Issue: https://github.com/dubinc/dub/issues/506
- Proposed by Steven Tey (Dub founder) in Dec 2023
- MVP shipped: login + config + shorten. Essentially became dubco.
- Closed: October 2024
- Status: No further development. The official CLI is the same 3-command MVP.

## User Pain Points

> "it feels nerve-wracking to build anything off a system I can't easily get my data out of if necessary." — GitHub Discussion #457 (link export)

> "It feels like we _really_ need at least an export of some kind" — GitHub Discussion #502 (import/export/backup)

> "We use Prometheus and Grafana to view analytics... would like to have all the analytics in one place" — GitHub Issue #488 (analytics export)

> Users requesting bulk actions (archive, delete, move) for links — GitHub Issue #2791

## Auth Method
- Type: Bearer token (API key)
- Env var convention: `DUB_API_KEY` (prefixed `dub_xxxxx`)
- Scopes: full_access or sending_access
- Key creation: Dashboard → API Keys page

## Demand Signals
- No Reddit/HN posts specifically requesting a Dub CLI
- GitHub issues/discussions show demand for export, bulk management, analytics archival
- Dub MCP server exists (Gitmaxd/dubco-mcp-server) — indicates AI agent interest
- Dub's own founder proposed a CLI (issue #506) but only an MVP was shipped

## Strategic Justification

**Why this CLI should exist:** The only Dub CLI (dubco, 24 stars) is unmaintained, has only 3 commands, and doesn't support any of the 14 API resources beyond link creation. Dub's paid plans have analytics retention limits (1-2 years) — a CLI with local SQLite storage solves a real data preservation problem that the dashboard can't. The API has rich filtering (30+ params on analytics alone) that is perfectly suited to CLI flags. No existing tool provides bulk link management, analytics archival, stale link detection, or link health checking from the terminal. This is genuinely greenfield — there is no competition to beat, only a void to fill.

## Target
- Command count: 48+ (cover all API endpoints) + 7 workflow commands = 55+
- Key differentiator: Local SQLite data layer with FTS5 search, analytics archival past retention limits, stale link detection, CSV import/export for link migration
- Quality bar: Grade A (80+/100)
