# SEC EDGAR CLI Brief

## API Identity
- **Domain:** Public company disclosures filed with the U.S. Securities and Exchange Commission. Covers every public-equity issuer, every mutual fund, every insider, every institutional investor managing >$100M in U.S. listed equities, since 1994.
- **Users:** Equity analysts, quant researchers, fintech builders, journalists, compliance/legal, academic finance, and increasingly LLM agents acting on behalf of any of the above.
- **Data profile:** Multi-billion-row archive of filings; per-company JSON for filings history (submissions), full XBRL fact graphs (companyfacts), and cross-company XBRL frames. Plus ~5M+ filings searchable as full text via EFTS, an Atom feed of the latest filings, and `Archives/edgar/data/...` raw filing trees (HTML, XML, exhibits). All free, no key.

## Reachability Risk
- **None.** All five key endpoints returned 2xx with the user-provided UA (`<redacted-test-user-agent>`):
  - `data.sec.gov/submissions/CIK0000320193.json` → 200, JSON
  - `data.sec.gov/api/xbrl/companyfacts/CIK0000320193.json` → 200, JSON
  - `data.sec.gov/api/xbrl/frames/us-gaap/AccountsPayableCurrent/USD/CY2024Q1I.json` → 200, JSON
  - `www.sec.gov/files/company_tickers.json` → 200, JSON (10K+ entries)
  - `efts.sec.gov/LATEST/search-index?q=...` → 200, JSON
  - `www.sec.gov/cgi-bin/browse-edgar?...&output=atom` → 200, Atom XML
- **Constraints:** UA header is **mandatory** (the SEC rejects bots with no UA or generic UAs with 403). Hard rate cap is **10 req/sec per host**. No API key. The `https://www.sec.gov/files/...` prefix needs the same UA but no other handshake.

## Top Workflows
1. **Look up a company's full filing history.** Ticker or name → CIK → all submissions, filterable by form type (10-K, 10-Q, 8-K, etc.) and date.
2. **Get the latest 8-K / 10-Q / 10-K and what it says.** Fetch the filing's index, list the primary documents, pull the exhibits, extract the cover-page facts.
3. **Track insider trading.** Form 3/4/5 transactions per CIK (the executive or director) and per issuer (the company), with transaction code, shares, price, post-trade ownership.
4. **Read XBRL financials precisely.** Single concept (e.g. `us-gaap:Revenues`) across all of a company's filings, or one concept across every public filer for a single quarter (XBRL frames).
5. **Full-text search across every filing.** Find every 10-K mentioning a phrase, in a date range, narrowed by form type or location.
6. **Stream the live feed.** Subscribe to filings as they come in, filter by form/CIK/keyword, alert on matches.

## Table Stakes
Every absorbed feature in this list exists in at least one competing tool and must be present in this CLI to claim parity:

- **edgartools** (dgunning/edgartools): typed objects for 20+ form types (10-K, 10-Q, 8-K, 13F, S-1, DEF 14A, Form 3/4/5, N-PORT, N-MFP, N-CSR, N-CEN, Schedule 13D/G, Form D, Form 144), XBRL balance sheet/income/cash-flow extraction, multi-period comparatives, automatic unit conversion, MCP integration, fresh-filing cache.
- **secedgar** (sec-edgar/sec-edgar): CLI for bulk-downloading filings by ticker, CIK, or daily window; quarterly/daily index walking.
- **bellingcat/EDGAR**: keyword/form search → CSV export.
- **sec-edgar-mcp** (stefanoamorelli/sec-edgar-mcp): MCP tools for CIK lookup, 10-K/10-Q/8-K retrieval, section extraction, XBRL-parsed financial statements, Form 3/4/5 insider transactions.
- **sec-edgar-toolkit** (stefanoamorelli): TS/JS + Python SDKs for filings, XBRL, insider, real-time retrieval.
- **sec-api** (janlukasschroeder/sec-api, paid): 150+ form types, query-builder, real-time stream of new filings, 13F holdings, exec comp, exhibits.
- **tumarkin/edgar**: local index + filing download.

## Data Layer
- **Primary entities** (each gets a SQLite table):
  - `companies` — CIK ↔ ticker(s) ↔ name ↔ SIC code ↔ state of incorporation ↔ fiscal-year-end (from submissions + company_tickers)
  - `filings` — accession number, CIK, form type, filing date, period of report, primary document URL, items list (for 8-K) (from submissions)
  - `xbrl_facts` — (CIK, taxonomy, tag, unit, period, value, accession) — denormalised XBRL facts from companyfacts (the queryable surface that competitors do not store locally)
  - `insider_txns` — Form 4 transactions parsed: insider name+CIK, issuer CIK, transaction date, shares, price, post-tx-ownership, transaction-code
  - `concepts` — taxonomy + tag + label + description (so users can `concepts list` instead of guessing tag names)
  - `latest_feed` — rolling cache of the Atom getcurrent feed for fast `watch`/`since`
- **Sync cursor:** Per-CIK `lastUpdated` (from submissions) + per-fact `filed` date. Atom feed has its own `updated` cursor.
- **FTS/search:** SQLite FTS5 over `filings.form_summary` (the 8-K item summaries, the 13D/G beneficial-owner names, the cover-page facts) and `companies.name`. The full-text-search endpoint at EFTS is the *online* path; the offline path is the FTS5 index built from synced filings.

## Codebase Intelligence
- **Source:** Combined research across edgartools README, sec-edgar-mcp README, the SEC's December 2025 EDGAR API Overview PDF, and live probes against `data.sec.gov`, `efts.sec.gov`, and `www.sec.gov/cgi-bin/browse-edgar`.
- **Auth model:** No key. Mandatory `User-Agent: <Name> <email>` header on every request (or 403). Default for this CLI: read from `SEC_EDGAR_USER_AGENT` env var, fall back to a config file, refuse to run without one.
- **Data model insight:** `data.sec.gov` and `www.sec.gov` are *two different hosts* — the XBRL JSON APIs live on `data.sec.gov`; the company-tickers map, the browse-edgar Atom feed, and the raw filing archives live on `www.sec.gov`. The CIK is always 10 digits, zero-padded — every API path requires that exact shape.
- **Rate limiting:** Hard cap of 10 req/sec per host across both hosts together (SEC enforces by IP). The CLI should rate-limit at 8 req/sec per host with simple jitter and exponential back-off on 429.
- **Architecture note:** XBRL "frames" lets us slice one concept across every filer for one period (e.g. "show me every public company's Q1 2024 AccountsPayableCurrent") — this is the cross-company aggregation pivot point that makes our local SQLite uniquely valuable.

## User Vision
- The user provided no specific product direction beyond an instruction to use `<redacted-test-user-agent>` as the mandatory SEC User-Agent for all live requests in this run. Treat that as a polite-research positioning hint — the CLI's default `SEC_EDGAR_USER_AGENT` value during dogfood/live testing will be that string.

## Source Priority
- **Single-source CLI.** No combo ordering to confirm. The primary surface is `data.sec.gov` (XBRL JSON) plus `www.sec.gov` (raw archives + Atom feed + company tickers) plus `efts.sec.gov` (full-text search). All three are official SEC hosts; treat them as a single source.

## Product Thesis
- **Name:** `sec-edgar-pp-cli` (binary), `sec-edgar` (slug, library directory).
- **Headline:** Every SEC filing, every XBRL fact, every insider trade — synced into a local SQLite store you can pivot, search, and watch offline.
- **Why it should exist:** Existing tools are Python-centric (edgartools, secedgar, sec-edgar-mcp) and force you to round-trip to the network for every question. None of them give you a single SQLite database where you can `JOIN insider_txns ON xbrl_facts` or run `SELECT cik FROM filings WHERE form='8-K' AND items LIKE '%2.05%'` without writing a Python loop. None of them give you a `watch` command that streams the live Atom feed and alerts on filter matches. None expose the same surface to an LLM agent via MCP with typed arguments and structured JSON output by default. A Go single-binary CLI with `--json`, `--select`, `--csv`, SQLite-backed sync, FTS5 search, and a parallel MCP server is the missing tool.

## Build Priorities
1. **Data layer + sync (Priority 0):** SQLite tables for companies, filings, xbrl_facts, insider_txns, concepts, latest_feed. Sync commands populate them from `data.sec.gov` and `www.sec.gov` with rate limiting and the mandatory UA.
2. **Absorbed endpoint mirrors (Priority 1):** `companies lookup`, `companies search`, `filings list`, `filings get`, `concepts list`, `facts get` (company concept), `facts company` (full company facts), `facts frame` (cross-company XBRL frames), `insider list` (per CIK), `insider company` (per issuer), `search` (EFTS full-text), `feed latest` (Atom). One typed Cobra command per surface, all with `--json` and `--dry-run`.
3. **Transcendence (Priority 2):** `watch` (stream Atom feed with `--form`, `--cik`, `--keyword` filters), `cross-section` (one XBRL concept across N companies in one period — pivots `frames`), `concept-history` (one concept across all filings for one company — pivots `companyconcept`), `insider-cluster` (companies with >N insiders buying in the same window), `form-cadence` (companies filing at unusual frequencies), `industry-bench` (XBRL frame slice + SIC-code group-by), and `sql` (raw SELECT against the local store, with FTS5 enabled).
4. **Polish (Priority 3):** README cookbook with realistic CIKs/tickers, `--select` recipes for the deeply-nested XBRL fact JSON, MCP server enrichment (hidden endpoint mirrors + code-orchestration pair for >50 tools), human-friendly error messages when CIK is malformed or UA is missing.
