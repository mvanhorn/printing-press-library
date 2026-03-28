---
title: "Visionary Research: Dub CLI"
type: feat
status: active
date: 2026-03-27
phase: "0"
api: "dub"
---

# Visionary Research: Dub CLI

## Overview

Dub.co is the modern link attribution platform for short links, conversion tracking, and affiliate programs. It powers 100M+ clicks and 2M+ links monthly, used by marketing teams at Framer, Perplexity, Superhuman, Twilio, and Buffer. The API (OpenAPI 3.0.3) exposes 48 endpoints across 14 resources, serving at `https://api.dub.co` with Bearer token auth.

The competitive landscape is wide open. The only existing CLI (sujjeee/dubco, 24 stars) has just 3 commands (login, config, link creation) and hasn't been updated since August 2024. There is no data tool, no analytics export CLI, no monitoring tool, and no TUI for Dub. The official SDKs (TS, Go, Python, Ruby) are Speakeasy-generated but there's no first-party CLI despite a closed feature request (dubinc/dub#506).

A CLI that combines full API coverage with local analytics storage, link export, and workflow commands for bulk management would fill a genuine gap -- especially given that Dub's paid plans have analytics retention limits (Pro: 1 year, Business: 2 years), making local archival genuinely valuable.

## API Identity

- **Domain:** Link management / marketing attribution / affiliate programs
- **Primary users:** Marketing teams, growth engineers, developers building referral/affiliate programs, startup founders managing links
- **Core entities:** Links, Domains, Tags, Folders, Analytics, Events, Customers, Partners, Commissions, Payouts, Bounties, QR Codes
- **Data profile:**
  - Write pattern: Mutable (links can be PATCH'd), with append-only events/analytics
  - Volume: Medium (links: thousands-tens of thousands per workspace; analytics: high volume click events)
  - Real-time: Webhooks (7 event schemas: LinkWebhookEvent, LinkClickedEvent, LeadCreatedEvent, SaleCreatedEvent, PartnerEnrolledEvent, PartnerApplicationSubmittedEvent, CommissionCreatedEvent). No WebSocket/SSE.
  - Search need: High (finding specific links by URL, tag, domain, folder; searching analytics by time range, geo, device)

## Usage Patterns (Top 5 by Evidence)

| # | Pattern | Evidence Score | Sources |
|---|---------|---------------|---------|
| 1 | **Analytics export/archival** | 5/10 | GitHub issue #488 (implemented web CSV export), analytics retention limits on paid plans, users want Prometheus/Grafana integration |
| 2 | **Bulk link management** | 5/10 | GitHub issue #2791 (bulk archive/delete/move), API has bulk create/update/delete endpoints (100 per call) |
| 3 | **Link export/backup** | 4/10 | GitHub discussions #457, #502 (vendor lock-in concerns, "nerve-wracking to build off a system I can't easily get my data out of") |
| 4 | **Short link creation from terminal** | 4/10 | dubco CLI (24 stars), dubco-mcp-server, official CLI MVP (dubinc/dub#506 closed) |
| 5 | **Domain management** | 3/10 | API has 5 domain endpoints (register, verify, configure), natural CLI workflow for multi-domain setups |

## Tool Landscape

### Forge/Platform CLIs (API Wrappers)
- **sujjeee/dubco** (24 stars, TypeScript) — Only 3 commands: login, config, link. Last commit Aug 2024. Unmaintained. No analytics, no domains, no tags, no bulk operations, no JSON output.

### Integration Tools
- **Gitmaxd/dubco-mcp-server** — MCP server for AI agents. Create/update/manage short links. Not a CLI.

### Official SDKs
- **dubinc/dub-ts** — TypeScript SDK (Speakeasy-generated)
- **dubinc/dub-go** — Go SDK (Speakeasy-generated)
- **dubinc/dub-python** — Python SDK
- **dubinc/dub-ruby** — Ruby SDK
- **dubinc/analytics** — Client-side JS SDK for conversion tracking

### Alternative UX / Data Tools
- **None.** No discrawl-class tools exist. No local storage, no offline search, no analytics archival tool. This is the gap.

### Forks / Community
- **willholmeswastaken/wub** — URL shortener inspired by dub.co (learning project)
- **Snazzah/stub** — Self-hostable fork of Dub

### Competing Platforms (not CLI tools)
- **Bitly** — Enterprise link management (no CLI)
- **Short.io** — Link management (no CLI)
- **Rebrandly** — Branded links (no CLI)
- **PIMMS** — Revenue attribution (no CLI)

**Key finding:** No competitor in the link management space has a real CLI. This is greenfield.

## Workflows

### 1. Campaign Link Creation (Bulk)
- **Steps:** Create 10-50 links for a marketing campaign with consistent tags, UTM params, and geo-targeting
- **Frequency:** Weekly for active marketing teams
- **Pain:** Dashboard is slow for bulk creation; API requires separate calls for tagging
- **Proposed:** `dub-pp-cli links bulk-create --csv campaign.csv --tag launch-2026 --domain dub.sh`

### 2. Analytics Snapshot & Archive
- **Steps:** Pull analytics for date range, aggregate by dimension (country, device, referer), save locally
- **Frequency:** Daily/weekly for reporting
- **Pain:** Dashboard analytics are ephemeral (retention limits), no offline access, rate-limited analytics API
- **Proposed:** `dub-pp-cli analytics snapshot --days 30 --group-by country,device` + `dub-pp-cli analytics export --format csv --since 2026-01-01`

### 3. Link Audit / Stale Link Detection
- **Steps:** List all links, find links with 0 clicks in N days, identify broken destinations
- **Frequency:** Monthly cleanup
- **Pain:** No built-in way to find stale/dead links from dashboard
- **Proposed:** `dub-pp-cli links stale --days 30` + `dub-pp-cli links health` (check destination URLs)

### 4. Domain Setup & Verification
- **Steps:** Register domain, verify DNS, configure SSL, set as default
- **Frequency:** Occasional but critical
- **Pain:** Multi-step process requiring dashboard + DNS provider
- **Proposed:** `dub-pp-cli domains add example.com --verify --default`

### 5. Partner/Affiliate Report
- **Steps:** List partners, get their link performance, calculate commissions
- **Frequency:** Monthly for affiliate program managers
- **Pain:** No single view across partners + links + commissions
- **Proposed:** `dub-pp-cli partners report --month 2026-03` + `dub-pp-cli payouts pending`

## Architecture Decisions

| Area | Decision | Rationale |
|------|----------|-----------|
| **Persistence** | SQLite with domain-specific tables for links, analytics snapshots, events | Analytics retention limits (1-2 years) make local archival genuinely valuable. Links are the core entity users search/filter. |
| **Real-time** | REST polling with page-based pagination | No WebSocket/SSE. Webhooks are push-only (need a server). REST polling with `?page=` cursor is the only option for sync. |
| **Search** | FTS5 on link URL, key, title, description | Users need to find links by partial URL, title, or description. Tags/folders provide structured filtering. |
| **Bulk** | Use API bulk endpoints (100 items/call) | API natively supports bulk create/update/delete. CLI should expose this via CSV import and --stdin. |
| **Cache** | Cache analytics locally, serve from SQLite for repeat queries | Analytics API is rate-limited (2 req/sec on Pro). Local cache avoids hitting limits for repeat queries. |

## Top 5 Features for the World

| # | Feature | Evidence | Impact | Feasibility | Uniqueness | Composability | Data Fit | Maintain | Moat | Total |
|---|---------|----------|--------|------------|------------|---------------|----------|----------|------|-------|
| 1 | **Local analytics archive + SQL queries** | 3 | 3 | 2 | 2 | 2 | 2 | 1 | 1 | **16/16** |
| 2 | **Bulk link management (CSV import/export)** | 2 | 3 | 2 | 2 | 2 | 2 | 1 | 0 | **14/16** |
| 3 | **Stale link detection + health check** | 2 | 2 | 2 | 2 | 2 | 2 | 1 | 1 | **14/16** |
| 4 | **Partner/affiliate reporting** | 1 | 2 | 2 | 2 | 2 | 2 | 1 | 1 | **13/16** |
| 5 | **Full link export/backup with restore** | 2 | 2 | 2 | 2 | 1 | 2 | 1 | 0 | **12/16** |

All 5 are must-haves (score >= 12).

## Sources

- Dub OpenAPI spec: https://api.dub.co (OpenAPI 3.0.3, 48 endpoints)
- Dub API docs: https://dub.co/docs/api-reference/introduction
- Dub rate limits: https://dub.co/docs/api-reference/rate-limits
- sujjeee/dubco: https://github.com/sujjeee/dubco (24 stars, last commit Aug 2024)
- dubco MCP server: https://github.com/Gitmaxd/dubco-mcp-server
- Export links discussion: https://github.com/dubinc/dub/discussions/457
- Import/export/backup discussion: https://github.com/dubinc/dub/discussions/502
- Analytics export issue: https://github.com/dubinc/dub/issues/488
- Bulk link management issue: https://github.com/dubinc/dub/issues/2791
- CLI feature request: https://github.com/dubinc/dub/issues/506 (closed Oct 2024)
