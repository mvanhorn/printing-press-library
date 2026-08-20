# SEC EDGAR CLI Brief

## API Identity
- Domain: SEC EDGAR — public corporate-disclosure filings (10-K, 10-Q, 8-K, Form 3/4/5, DEF 14A, 13D/G, 13F, S-1, N-PORT, XBRL company facts).
- Users: Equity research agents (LODESTAR `/$research`, `/$recheck`), quant pipelines, fundamental analysts.
- Data profile: Mixed — structured JSON (submissions, XBRL companyfacts, EDGAR full-text search), semi-structured XML (Form 3/4/5 primary docs), unstructured HTML (10-K/10-Q/8-K bodies, 100 KB–10 MB each). Append-only by accession number; immutable once filed.

## Spec Status
- No OpenAPI spec. SEC publishes prose documentation only. Canonical doc pages:
  - `https://www.sec.gov/search-filings/edgar-application-programming-interfaces` (data.sec.gov endpoints)
  - `https://www.sec.gov/edgar/sec-api-documentation` (overview; returned 403 to WebFetch but cited across community wrappers)
  - `https://www.sec.gov/os/accessing-edgar-data` (fair-access policy)
- Behavior must be inferred from working community wrappers (edgartools, secedgar) and SEC's published fair-access notes.

## Reachability Risk
- **Medium.** SEC actively blocks misbehaving clients. Two distinct failure modes confirmed in community issues:
  1. **403 Forbidden** when User-Agent header is missing or generic — first request fails outright. Confirmed in `jadchaar/sec-edgar-downloader#77`, `areed1192/python-sec#6`, and finxter post "Solving Response [403] HTTP Forbidden Error: Scraping SEC EDGAR".
  2. **429 + 10-minute IP block** when sustained rate exceeds SEC's published 10 req/sec ceiling. Confirmed in `sec-edgar/sec-edgar#194` ("Strategies to avoid rate-limiting errors") and `joeyism/py-edgar#24`. SEC's own filergroup announcement confirms 10 req/sec cap; we target ≤2 req/sec to stay well clear.
- Mitigation: required `COMPANY_PP_CONTACT_EMAIL` env var → User-Agent `lodestar-edgar-pp-cli <email>`; refuse-to-run if unset; 2 req/sec token bucket; exponential backoff on 429.

## Top Workflows
1. Pull LODESTAR PRIMARY-SOURCES bundle for a ticker (10-K + 4× 10-Q + 8-Ks 90d + Form 4s 12mo + DEF 14A) — one call, structured JSON out.
2. Insider transaction summary with senior-officer flagging and **code-S (discretionary sale) vs. code-F (RSU tax withholding) discrimination**. Code F is the most common transaction code on Form 4 and is non-directional — collapsing the two would destroy the signal LODESTAR depends on.
3. Per-form-type atomic pull (`filings --type 10-K|10-Q|8-K|4|DEF+14A`).
4. Local SQLite sync per ticker (tiered TTLs: companyfacts ~24h, submissions ~6h, individual filings immutable).
5. FTS5 full-text search across cached filing bodies (cheaper and offline-friendly vs. `efts.sec.gov` round-trips).

## Table Stakes (from competitor scan)
- Ticker → CIK resolution.
- Form 3/4/5 parsing into typed transactions with all Table I/II transaction codes (A, D, F, G, I, M, P, S, V, X).
- XBRL companyfacts extraction (revenue, EPS, segments) without raw XBRL exposure.
- Multi-form retrieval by ticker + form type + date range.
- HTML-to-text body extraction for 10-K / 10-Q (the universally-cited pain point).
- Local caching to avoid re-hitting SEC.

## Data Layer
- Primary entities: `filings` (accession_no PK, cik, form_type, filed_at, primary_doc_url, body_text), `companies` (cik PK, ticker, name, sic), `insider_transactions` (accession_no FK, reporter_cik, is_senior_officer, transaction_code, shares, price, value, is_discretionary), `xbrl_facts` (cik, concept, unit, period, value).
- Sync cursor: per-CIK `last_seen_accession` from submissions index.
- FTS5: virtual table over `filings.body_text` for parseable forms (10-K/Q/8-K).

## Codebase Intelligence
- edgartools architecture (from README inspection, not DeepWiki): single `Filing` object wraps each accession; per-form-type subclasses (`TenK`, `EightK`, `Form4`) attach domain-specific parsers; XBRL parsed lazily; transport layer is `httpx` with a global rate limiter and a required User-Agent set via `set_identity()` — calls fail loudly if unset. Form 4 parser surfaces transaction code, acquired/disposed flag, shares, price-per-share, and post-transaction ownership. This is the closest existing reference for what `edgar-pp-cli`'s Go layer needs to replicate.

## User Vision
LODESTAR conviction-research framework. The CLI's target consumer is a Claude
Code agent running LODESTAR's /$research and /$recheck skills. Token efficiency >
human readability. The 3 compound commands (primary-sources, insider-summary,
filings) are the LODESTAR-specific value-add. Form 4 code-S-vs-code-F
distinction is mandatory. AAPL regression test: insider-summary AAPL
--senior-only must flag the CEO a recent CEO discretionary sale as discretionary.
8-K enumeration must surface post-2026-05-08 filings. SEC compliance
non-negotiable: lodestar-edgar-pp-cli <email> User-Agent, refuse-if-unset on
COMPANY_PP_CONTACT_EMAIL, <=2 req/sec, adaptive 429 backoff.

## Sanctioned Endpoints (locked by user — 6 endpoints total)
1. https://www.sec.gov/cgi-bin/browse-edgar
2. https://data.sec.gov/submissions/CIK{cik}.json
3. https://data.sec.gov/api/xbrl/companyfacts/CIK{cik}.json
4. https://www.sec.gov/Archives/edgar/data/{cik}/{accession}/
5. https://efts.sec.gov/LATEST/search-index
6. https://www.sec.gov/files/company_tickers.json — ticker→CIK resolution (approved post-research, cache 24h)

## Out of Scope (LODESTAR handles elsewhere via external complementary tooling)
- Federal Register
- regulations.gov dockets
- Activist short-seller scans
- NASDAQ short interest

## Endpoint Decisions (resolved post-research)
- `company_tickers.json` — **APPROVED by user as endpoint #6** (see Sanctioned Endpoints above). Cache TTL 24h.
- `company_tickers_exchange.json` — declined; exchange info is nice-to-have, not required by the LODESTAR workflows.
- No other endpoints surfaced as needed. The six sanctioned endpoints cover every workflow.

## Product Thesis
- Name: edgar-pp-cli
- Why it should exist: LODESTAR's research agent currently has to either (a) chain 5–10 WebFetch calls per ticker against SEC, paying full HTML-token cost on every 10-K body and re-deriving the User-Agent/rate-limit dance every session, or (b) shell out to a Python library that doesn't ship as a single binary. A Go CLI gives the agent one binary that handles SEC fair-access compliance once (User-Agent enforcement, 2 req/sec limiter, 429 backoff), caches structured outputs in SQLite (token-cheap re-reads), and emits compact JSON tuned for an LLM consumer — not human-readable HTML. The three compound commands (`primary-sources`, `insider-summary`, `filings`) collapse what is currently ~12 WebFetch round-trips per ticker into one invocation, and the Form 4 S/F discrimination is the specific signal LODESTAR needs that no generic competing CLI emits.

## Build Priorities
1. Foundation: SQLite store with tiered TTLs, User-Agent compliance + refuse-if-unset, 2 req/sec limiter, typed exit codes, `--compact`/`--json`/`--since`.
2. `filings` atomic command (all 5 form types via browse-edgar + submissions index).
3. `insider-summary` with senior-officer flagging from Form 4 reportingOwner.officerTitle and S/F transaction-code discrimination.
4. `primary-sources` bundle command (composes 1–3 internally + XBRL companyfacts + DEF 14A).
5. `sync` (incremental per-CIK pull driven by submissions cursor) + FTS5 `search` over cached bodies.

## Absorb Sources (for Phase 1.5)
| Name | URL | Lang | Stars (verified) | Last activity | Key capabilities |
|------|-----|------|------------------|---------------|------------------|
| edgartools | https://github.com/dgunning/edgartools | Python | 2,100 | 2026-05-12 (v5.31.1) | Form 4 parsing w/ all transaction codes, XBRL companyfacts, per-form-type object model, built-in MCP server, User-Agent enforcement pattern (`set_identity()`) |
| secedgar | https://github.com/sec-edgar/sec-edgar | Python | 1,400 | active (sustainable per snyk) | Bulk filing download by ticker/date, async retrieval, daily-index iteration, rate-limit handling patterns |
| sec-edgar-mcp | https://github.com/stefanoamorelli/sec-edgar-mcp | Python | 265 | 2026-01-25 (v1.0.8) | MCP tool surface design (CIK lookup, company facts, 10-K/Q/8-K section extraction, Form 3/4/5) — direct reference for agent-facing tool ergonomics |
| sec-edgar-toolkit | https://github.com/stefanoamorelli/sec-edgar-toolkit | Py + TS | unverified | active | Comprehensive 10-K/Q/8-K parsing, XBRL extraction across two language SDKs — cross-language API shape reference |
| palafrank/edgar | https://github.com/palafrank/edgar | Go | unverified low | unverified | Go-language reference for filing retrieval and parsing — useful as a starting-point comparison for Go idioms even though scope is narrow |
| jadchaar/sec-edgar-downloader | https://github.com/jadchaar/sec-edgar-downloader | Python | unverified | active | Bulk downloader; key issues (#77, #24) inform our rate-limit + User-Agent handling |
| sec-api.io (commercial) | https://sec-api.io / https://github.com/janlukasschroeder/sec-api | TS | unverified | active | Reference for *output shape* of parsed Form 4 JSON (acquisition/disposition flags, share counts, post-transaction holdings) — paid service, not absorbable code, but valuable schema reference |

## Reachability Risk evidence
- `sec-edgar/sec-edgar` Discussion #194 — "Strategies to avoid rate-limiting errors": once a User-Agent is added, 403s convert to 429s; pattern is well-known.
- `jadchaar/sec-edgar-downloader` Issue #77 — "403 Forbidden is Back!": confirms SEC periodically tightens enforcement; recurring issue across years.
- `joeyism/py-edgar` Issue #24 — "Rate limit edgar requests": documents the 10 req/sec ceiling and 10-minute IP block.
- `areed1192/python-sec` PR #6 — "fix SEC error 403: add user-agent header": confirms User-Agent is the #1 cause of new-developer 403s.
- SEC filergroup announcement: `https://www.sec.gov/filergroup/announcements-old/new-rate-control-limits` — official 10 req/sec policy.

## Pain Points (from user research, for transcendence ideation)
1. **CIK ↔ ticker resolution friction.** Every workflow starts with a ticker, but every SEC endpoint takes a 10-digit zero-padded CIK. Wrappers all reinvent this. The CLI should make it invisible.
2. **Form 4 transaction-code semantics are buried.** Naive aggregators sum shares across all codes and mistake routine RSU tax withholding (code F, mechanical) for active selling (code S, signal). Multiple community parsers expose the code but leave interpretation to the caller — LODESTAR needs the CLI to surface `is_discretionary` directly.
3. **10-K / 10-Q HTML body extraction is the universal nightmare.** Filings range 100 KB–10 MB of nested HTML with inline XBRL tags; section boundaries (Item 1A Risk Factors, MD&A) are inconsistent across filers. Every wrapper has its own partial solution; none is perfect. For a token-conscious agent consumer, structured section extraction with byte offsets is far more valuable than dumping raw HTML.
4. **User-Agent / rate-limit footgun.** Confirmed across at least 4 distinct repo issue threads; new developers consistently get 403'd on first run. Handling this once at the CLI boundary is high-value.
5. **Accession-number formatting inconsistency.** SEC stores accessions as `0000-320193-22-000049` in some endpoints and `000032019322000049` (no dashes) in URL paths. Trips up nearly every new integration.
