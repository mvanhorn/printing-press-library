# nonprofit-explorer Absorb Manifest

## Source Tools Surveyed

| Tool | Type | Role |
|---|---|---|
| ProPublica Nonprofit Explorer (web) | Website | Canonical search/profile UI over the same API |
| ProPublica Nonprofit Explorer API v2 | REST/JSON, 2 endpoints | The data source this CLI wraps |
| IRS EO BMF + SOI 990 extracts | Bulk CSV downloads | Upstream data ProPublica parses; no query API |
| Urban Institute NCCS NTEE-CC table | Classification reference | Embedded (633 codes) for human cause-area names |

No existing dedicated CLI for this API was found; the transcendence bar is the
website itself plus raw curl.

## Absorbed Features (match the site / raw API)

| # | Feature | Source | Our Command | Added Value |
|---|---|---|---|---|
| 1 | Full-text org search + filters | Site search | `search <query> --state --ntee --c-code` | Ranked table with full NTEE-CC names, --agent JSON, zero-match exits 0 |
| 2 | Org profile + filings | Site org page | `org <ein-or-name>` | Latest-990 summary inline (rev/exp/assets/liab/net + PDF) |
| 3 | Raw endpoint access | curl | `organizations`, `search-json` | Local EIN validation (API 200-stubs malformed EINs), cache, envelopes |

## Novel Features (not available in any surveyed tool)

| # | Feature | Command | Description |
|---|---|---|---|
| 1 | Name-or-EIN resolution | `org` / `filings` / `financials` / `people` / `compare` | Any EIN argument also accepts a nonprofit name; auto-resolves to the top search match with a stderr note or `resolved` JSON envelope field |
| 2 | Financial trajectory | `financials` | Year-by-year revenue/expense/net/assets with YoY revenue change, latest-year revenue composition, personnel-cost share of expenses |
| 3 | Side-by-side comparison | `compare` | Latest-990 comparison of N organizations, names and EINs mixed freely |
| 4 | NTEE-CC naming | `search` / `org` | Embedded 633-code table renders full cause-area names (T23 = Private Operating Foundations) |
| 5 | Officer-comp aggregates | `people` | Officer compensation total + share of expenses, other salaries, payroll taxes, pro fundraising by year, with per-year 990 PDF links |

## Dogfood

Live Phase 5 matrix: 91 passed / 0 failed / 58 skipped (level full, no auth
required) — see `../proofs/phase5-acceptance.json`.
