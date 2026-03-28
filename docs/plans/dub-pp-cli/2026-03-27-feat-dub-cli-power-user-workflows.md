---
title: "Power User Workflows: Dub CLI"
type: feat
status: active
date: 2026-03-27
phase: "0.5"
api: "dub"
---

# Power User Workflows: Dub CLI

## API Archetype: Content + CRM Hybrid

Links are the core content entity. Partners/customers/commissions form a CRM layer. Workflows target both: link lifecycle management + affiliate program reporting.

## Workflow Ideas (13 total, validated against spec)

| # | Workflow | API Endpoints | Frequency | Pain | Feasibility | Uniqueness | Total |
|---|---------|--------------|-----------|------|-------------|------------|-------|
| 1 | **analytics snapshot** | GET /analytics (groupBy: count, timeseries, countries, devices, browsers, referers, top_links) | Daily/Weekly (3) | High - retention limits (3) | Easy (3) | No tool (3) | **12** |
| 2 | **links export** | GET /links (paginate via startingAfter) | Monthly (2) | High - vendor lock-in (3) | Easy (3) | No tool (3) | **11** |
| 3 | **links bulk-create** | POST /links/bulk (100/call) | Weekly (2) | High (3) | Easy (3) | No tool (3) | **11** |
| 4 | **events tail** | GET /events (sortBy=timestamp, poll) | Daily monitoring (3) | Medium (2) | Easy (2) | No tool (3) | **10** |
| 5 | **analytics compare** | 2x GET /analytics with different start/end | Weekly (2) | Medium (2) | Easy (3) | No tool (3) | **10** |
| 6 | **links stale** | GET /links + GET /analytics per link | Monthly (2) | Medium (2) | Medium (2) | No tool (3) | **9** |
| 7 | **links health** | GET /links + HTTP HEAD each destination | Monthly (1) | High - broken links (3) | Medium (2) | No tool (3) | **9** |
| 8 | **links import** | POST /links/bulk (from Bitly/Short.io CSV) | One-time (1) | High for switchers (3) | Medium (2) | No tool (3) | **9** |
| 9 | **links bulk-tag** | PATCH /links/bulk | Monthly (1) | Medium (2) | Easy (3) | No tool (3) | **9** |
| 10 | **partners report** | GET /partners + /partners/analytics + /commissions | Monthly (1) | Medium (2) | Medium (2) | No tool (3) | **8** |
| 11 | **analytics top** | GET /analytics?groupBy=top_links | Daily (3) | Low (1) | Easy (3) | Dashboard does it (0) | **7** |
| 12 | **domains verify** | GET /domains/status | Occasional (1) | Medium (2) | Easy (3) | Dashboard (0) | **6** |
| 13 | **payouts pending** | GET /payouts | Monthly (1) | Low (1) | Easy (3) | Dashboard (0) | **5** |

## Top 7 for Implementation

### 1. `snapshot` (Score: 12/12)
Pull analytics for a time range across all dimensions, save to local SQLite.
```
dub-pp-cli snapshot --days 30
dub-pp-cli snapshot --start 2026-01-01 --end 2026-03-27
```
Calls GET /analytics with groupBy=count,timeseries,countries,devices,browsers,referers,top_links. Stores results locally. Serves future queries from SQLite without hitting rate-limited analytics API.

### 2. `export` (Score: 11/12)
Export all links to CSV or JSON. Paginates through entire workspace.
```
dub-pp-cli export --format csv > links.csv
dub-pp-cli export --format json --tag launch-2026
```
Calls GET /links with startingAfter pagination. Outputs all link fields including tags, geo-targeting, UTM params.

### 3. `import` (combined with bulk-create, Score: 11/12)
Import links from CSV (including Bitly/Short.io format migration).
```
dub-pp-cli import --csv campaign.csv --tag launch-2026 --domain dub.sh
dub-pp-cli import --csv bitly-export.csv --format bitly
```
Reads CSV, maps columns to Dub link schema, calls POST /links/bulk in batches of 100.

### 4. `tail` (Score: 10/12)
Stream recent events (clicks, leads, sales) to terminal.
```
dub-pp-cli tail
dub-pp-cli tail --event clicks --domain dub.sh
dub-pp-cli tail --event sales --json | jq '.amount'
```
Polls GET /events with sortBy=timestamp, displays new events as they appear. Domain/event type filtering via query params.

### 5. `compare` (Score: 10/12)
Compare analytics between two time periods side-by-side.
```
dub-pp-cli compare --this-month --last-month
dub-pp-cli compare --start1 2026-01 --start2 2026-02 --group-by country
```
Two GET /analytics calls, computes deltas, shows +/- percentage for clicks, leads, sales.

### 6. `stale` (Score: 9/12)
Find links with zero clicks in N days.
```
dub-pp-cli stale --days 30
dub-pp-cli stale --days 90 --tag campaign-q1
```
Queries local SQLite if synced, or calls GET /links + GET /analytics per link. Flags links with 0 clicks for review or archival.

### 7. `health` (Score: 9/12)
Check if link destinations are still alive.
```
dub-pp-cli health
dub-pp-cli health --tag production --fix
```
GET /links, then HTTP HEAD each destination URL. Reports 4xx/5xx/timeout. Optional --fix to archive broken links.

## Naming Pass

| API-oriented | User-oriented (chosen) | Why |
|-------------|----------------------|-----|
| analytics-archive | `snapshot` | Users say "take a snapshot" not "archive analytics" |
| links-export | `export` | Clean verb, complements `import` |
| bulk-create-links | `import` | Users import campaigns, they don't "bulk create" |
| events-stream | `tail` | Unix convention, every dev knows `tail -f` |
| analytics-diff | `compare` | Users compare periods, not diff analytics |
| zero-click-links | `stale` | Matches the question: "which links are stale?" |
| destination-check | `health` | Standard CLI convention (like `doctor` for config) |

## Validation Notes

- **analytics snapshot**: Validated. GET /analytics supports `groupBy` param with values: count, timeseries, top_links, top_urls, referers, refererUrls, countries, cities, regions, continents, devices, browsers, os, triggers. Also supports `start`, `end`, `interval`, `timezone` params.
- **links export**: Validated. GET /links supports `startingAfter` cursor pagination, `sortBy`, `sortOrder`, `pageSize` (up to 100). Can filter by `domain`, `tagId`, `tagIds`, `tagNames`, `folderId`, `search`.
- **import**: Validated. POST /links/bulk accepts array of link objects.
- **tail**: Validated. GET /events supports `sortBy`, `sortOrder`, `page`, `limit`. Can filter by `event` (clicks/leads/sales), `domain`, `linkId`, `country`, etc.
- **compare**: Validated. GET /analytics supports `start` and `end` date params.
- **stale**: Requires combining links list + per-link analytics. GET /analytics supports `linkId` param.
- **health**: GET /links returns `url` (destination). HTTP HEAD check is local, no API needed.
