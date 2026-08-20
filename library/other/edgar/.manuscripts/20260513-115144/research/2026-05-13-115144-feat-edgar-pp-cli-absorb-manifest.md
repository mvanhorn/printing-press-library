# edgar-pp-cli — Absorb Manifest

Source: SEC EDGAR public-company filings. Built from Phase 1 research brief at
`2026-05-13-115144-feat-edgar-pp-cli-brief.md`. Six sanctioned endpoints; no
Federal Register / regulations.gov / activist-short / NASDAQ-SI scope.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Ticker → CIK resolution | edgartools `find()` / sec-edgar-mcp `lookup_company_cik` | `edgar-pp-cli companies lookup <TICKER>` — pulls company_tickers.json, caches 24h in SQLite | Cache makes subsequent lookups offline & instant; --json emits CIK + name + SIC |
| 2 | Per-form-type filings pull | edgartools `c.get_filings(form="10-K")` | `edgar-pp-cli filings <TICKER> --type 10-K [--since DATE] [--count N]` | Atomic, agent-native flags, exit codes, --since on every command |
| 3 | 10-K parsing (cover + items) | edgartools `TenK` class | `edgar-pp-cli filings <TICKER> --type 10-K --compact` extracts cover page + Item 5 + Item 9B | --compact returns LODESTAR-relevant fields only (shares outstanding by class, warrants, convertibles, preferred, pending litigation) |
| 4 | 10-Q parsing | edgartools `TenQ` class | `--type 10-Q --count 4 --compact` returns last 4 quarters with MD&A risk-factor diffs | Cross-quarter diff is not in any wrapper; built from cached body_text |
| 5 | 8-K parsing | edgartools `EightK` class | `--type 8-K --since 90d --compact` returns filing_date + Item code + 1-line summary per filing | Compact summary is built for LLM consumption, not human reading |
| 6 | Form 4 parsing | edgartools `Form4`, sec-edgar-mcp `get_insider_trades` | `edgar-pp-cli filings <TICKER> --type 4 --since 12mo` | Typed insider transactions with all Table I/II codes (A,D,F,G,I,M,P,S,V,X), parsed to columns |
| 7 | DEF 14A parsing | edgartools `DEF14A` (limited) | `--type DEF+14A --compact` extracts exec comp structure, board composition, auditor, related-party txns | LODESTAR-specific field selection no wrapper emits as compact JSON |
| 8 | Full-text search | edgartools `Filings.search()`, sec-edgar-mcp `full_text_search` | `edgar-pp-cli search <QUERY> [--form TYPE] [--since DATE]` via efts.sec.gov | Mirrors results to local FTS5 for offline re-query |
| 9 | XBRL company facts | edgartools `Company.facts()`, sec-edgar-mcp `get_company_facts` | `edgar-pp-cli companyfacts <TICKER> [--concept Revenues]` | Concept-filtered, --json, cached 24h |
| 10 | Bulk download per CIK | secedgar | `edgar-pp-cli sync <TICKER>` — incremental pull driven by submissions cursor | Incremental not bulk; respects 2 req/sec; resumable |
| 11 | Accession-number normalization | edgartools (internal) | `edgar-pp-cli accession <ID>` — normalizes `0000-320193-22-000049` ↔ `000032019322000049` | First-class command, both directions, --json |
| 12 | User-Agent enforcement | edgartools `set_identity()` | `COMPANY_PP_CONTACT_EMAIL` env var; refuse-to-run if unset; built into every request | Single config point; clear error message; documented |
| 13 | Rate limiting (≤10 req/s SEC cap) | edgartools internal limiter | Token bucket at 2 req/sec sustained; adaptive 429 backoff | Conservative default (well under SEC's 10 req/s ceiling) |
| 14 | Submissions index fetch | edgartools `Company` constructor, sec-edgar-mcp `get_submissions` | `edgar-pp-cli companies submissions <TICKER>` | Cached 6h; --json emits list of (accession, form, filed_at, primary_doc) |
| 15 | Doctor / health check | sec-edgar-mcp implicit (errors at startup) | `edgar-pp-cli doctor` — checks UA env var, reachability probe, SQLite open, FTS5 enabled | First-class command; typed exit codes |

15 absorbed features. Every one shipped with: `--json`, `--compact` (where applicable), `--since`, typed exit codes (0/2/3/4/5/7), SQLite caching with tiered TTLs, offline-first re-reads.

## Transcendence (only possible with our approach)

Populated by Step 1.5c.5 novel-features subagent. Full audit trail (customer model, all 16 pre-cut candidates, kill rationale) in `2026-05-13-115144-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Recheck delta cursor | `since <TICKER> --as-of TS` | 9/10 | Requires local SQLite `filings` table with `last_seen_accession` cursor; no SEC endpoint returns "what's new since DATE" |
| 2 | 8-K material-item enumeration | `eightk-items <TICKER> --since DATE --material-only` | 9/10 | Requires 8-K body parse for Item-number taxonomy (1.01, 2.02, 4.02, 5.02, 7.01, 8.01, 9.01); SEC endpoint returns the body, not the structured Item list with material-vs-exhibits classification |
| 3 | Insider follow-through join | `insider-followthrough <TICKER>` | 8/10 | Requires local cross-entity join `insider_transactions` (S, senior, ≥$1M) × `filings` (8-K within +90d); no API call returns this correlation |
| 4 | Cross-sectional XBRL pivot | `xbrl-pivot --tickers ... --concepts ...` | 8/10 | Multi-ticker pivot with concept-alias resolution (Revenues ↔ RevenueFromContractWithCustomer... ↔ SalesRevenueNet); companyfacts is single-ticker, no aliasing |
| 5 | Item-scoped section extraction | `sections <TICKER> --form 10-K --items 1A,7,7A` | 8/10 | Byte-offset Item-anchor parser emits requested items as compact JSON; SEC returns the full 100KB-10MB HTML body undifferentiated |
| 6 | Ownership-cross enumeration | `ownership-crosses <TICKER>` | 7/10 | Submissions index filter to 13D/G/-A + cover-page percent parse; service-specific 5%-cross taxonomy not exposed by community wrappers |
| 7 | Governance red flags | `governance-flags <TICKER>` | 8/10 | Composes three independent service-specific signals (8-K Item 4.01 auditor change, 4.02 restatement, NT-10-K late filing); no wrapper bundles these |
| 8 | Offline FTS over cached bodies | `fts <QUERY> --ticker TICK --form 10-K` | 7/10 | FTS5 virtual table over cached `filings.body_text` with byte-offset snippets; never re-hits efts.sec.gov; token-cheap for repeated queries |

## Cross-check items for novel-features brainstorm

Two Python MCPs already exist in this niche (edgartools-MCP, sec-edgar-mcp). The novel features must be defensibly different from what those MCPs already do. Specifically:

- **Compound bundles** (`primary-sources`) — neither competing MCP composes 10-K + 10-Q×4 + 8-K90d + Form4-12mo + DEF14A into one structured call. This is LODESTAR-specific shape.
- **Form 4 S/F discrimination** — edgartools exposes the code; sec-edgar-mcp surfaces transactions but does not flag `is_discretionary` directly. LODESTAR needs this column, not the raw code.
- **Senior-officer flagging** — derived from `reportingOwner.officerTitle` matching CEO/CFO/COO/CTO/Chairman. No competing MCP/CLI does this aggregation.
- **Local SQLite + FTS5** — neither competing MCP caches body text for offline re-query. Token-economy advantage for the agent consumer.
- **Cross-quarter MD&A diff** — neither MCP computes risk-factor changes between consecutive 10-Q filings.
- **Single Go binary** — neither competing tool is single-binary, statically-linkable, no-runtime-deps.

These are NOT proposed novel features; they are the territory the novel-features subagent should ground its candidates in. The subagent will produce the actual transcendence rows.
