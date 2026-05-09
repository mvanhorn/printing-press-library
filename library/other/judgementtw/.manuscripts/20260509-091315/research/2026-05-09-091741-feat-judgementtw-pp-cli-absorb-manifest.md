# judgementTW CLI — Absorb Manifest

Two surfaces, one local store, agent-native everywhere.

- **Source A (peer):** FJUD `judgment.judicial.gov.tw` — judgment search + detail (41 courts, 5 case types)
- **Source B (peer):** FJUDKM `fjudkm.judicial.gov.tw` — judicial knowledge base (462 topics + full-text search)
- **Auth:** None (per Phase 1.6 user decision: official open-data API path skipped entirely)
- **Transport:** standard_http (probe confirmed both sites)

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|--------------------|-------------|
| 1 | Search judgments by free-text keyword | samttoo22/judgement-scrawler (Selenium) | POST `/FJUD/Default_AD.aspx` → parse iframe q-token → GET `qryresultlst.aspx?ty=JUDBOOK&q=<token>` | Stdlib HTTP, no Selenium; `--json`, `--select`, `--csv`, `--limit`, `--page` |
| 2 | Filter by court (multi-select) | scrawler accepts single court | `--court TPS,TPH,TPD,...` (typed enum across all 41 courts) | All 41 courts mapped; multi-select supported |
| 3 | Filter by case type | scrawler 5 types | `--type criminal,civil,administrative,disciplinary,constitutional` | Long-form names + short codes (M/V/A/P/C) |
| 4 | Filter by year + case-character + number range | scrawler partial | `--year`, `--char`, `--no`, `--no-end` | Composable, agent-friendly |
| 5 | Filter by date range | scrawler not exposed | `--from YYY/MM/DD --to YYY/MM/DD` (民國) | Bidirectional 民國 ↔ Gregorian (`date convert` helper folded into flag parser) |
| 6 | Filter by case reason / verdict / body keyword | website only | `--reason`, `--verdict`, `--keyword` | Three independent text fields |
| 7 | Fetch judgment by JID | Judicial-OD API (0–6 AM only) | `get <jid>`: GET `/FJUD/data.aspx?ty=JD&id={jid}&ot=in` | 24/7 access via website (no time-window) |
| 8 | Fetch judgment with PDF attachment | manual via website | Auto-download `/FILES/{court}/{jid}.pdf` when present (or `--with-pdf`) | One-step bundle |
| 9 | Bulk download (>500 records) | scrawler "past 20 years" | `sync` with paginated fetch + adaptive rate limit + resume cursor | Idempotent, agent-cursor pagination, JSON/text/CSV; respects `change_log` |
| 10 | Persist corpus locally | scrawler `.txt` files | SQLite store with FTS5 (CJK-friendly trigram tokenizer) | Queryable; cross-judgment joins; offline search |
| 11 | Result list pagination | website iframe pager | `--page N` + `--limit N` + `sync --all` | Agent-native cursor |
| 12 | CSV export | not in any incumbent | `--csv` from any list command + `export <table>` | Universal export (also `--json`) |
| 13 | Browse 462 knowledge topics | FJUDKM website | `knowledge topics` lists all; `knowledge browse <id>` walks tree | JSON tree navigation |
| 14 | Per-topic case-commentary list | FJUDKM website | `knowledge topic <id>` returns `index_doc` items | Replayable, JSON-extractable |
| 15 | Single case-commentary detail | FJUDKM website | `knowledge get <par>` fetches commentary | Replayable, structured |
| 16 | Full-text knowledge search | FJUDKM website | `knowledge search <query>` POSTs to `searcher.aspx` | Composable with judgment search |
| 17 | Statute-citation extraction | none (Lawsnote partial) | At sync, regex-extract statute references from `理由` text into `citations` table | Powers novel features #3, #4, #9, #10 |
| 18 | Sentencing extraction | none | At sync, regex-extract 主文 sentence patterns (有期徒刑、拘役、罰金) into `sentences` table | Powers novel feature #5 |

All Status `(shipping)` — none of the above ship as stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence | Persona-served |
|---|---------|---------|-------|--------------|----------|----------------|
| 1 | Watch a specific case for new rulings | `watch case <jid-pattern>` | 8/10 | Polls FJUD search by court+案號 root; diffs returned JIDs against local `change_log`; prints additions | Yi-Chen frustration: no RSS/digest; brief Top Workflow #2 | Yi-Chen Lin, Mei-Ling Chen |
| 2 | Saved-query daily digest | `watch query <name> --terms <q> [--since YYY/MM/DD]` | 8/10 | Stored named query in local `watchlist` table; on each run, prints JIDs newer than cursor | Brief Top Workflow #2; weekly ritual for 3 personas | Mei-Ling, Yi-Chen, Wu |
| 3 | Statute-citation graph | `cites statute <code> [--article N]` | 9/10 | Local query: `judgments` × `citations` joined by court/year | Brief Build Priority 7; competitor gap (only Lawsnote does this, currently chilled by 2024 verdict) | Mei-Ling, Wu |
| 4 | Reverse-citation precedent lookup | `cited-by <jid>` | 8/10 | Reverse index of `citations` table | Brief Top Workflow #4; explicit paralegal use | Mei-Ling, Wu |
| 5 | Sentencing distribution | `sentences --statute <code> [--court <ct>] [--year YYY]` | 7/10 | `sentences` table aggregation; histogram + min/median/max | Brief Build Priority 7; Wu's ritual; samttoo22 has no analytics | Wu, Mei-Ling |
| 6 | Case-character (字別) catalog | `case-types list [--court <ct>]` | 6/10 | Aggregation of `case_character` grouped by court; sample JID per char | Brief 字別 taxonomy; agent enum discovery | Mei-Ling, agent |
| 7 | Appeal-chain walker | `appeal-chain <jid>` | 7/10 | Joins court-hierarchy + 案號 root match against local store | Brief Top Workflow #4 | Mei-Ling |
| 8 | Privacy-purge sweeper | `purge --orphans` | 7/10 | Re-fetches synced JIDs; on `查無資料` deletes + writes audit-log | Brief Reachability §3 (ToS-required) | All |
| 9 | Knowledge ↔ judgment linker | `knowledge link <par>` | 6/10 | FJUDKM commentary → extract statute refs → match local `citations` | Only meaningful join across the two sources | Wu, Mei-Ling |
| 10 | Related-case discovery | `related <jid>` | 7/10 | Jaccard similarity over citation set, filtered to same court tier ±2 years | Brief Build Priority 7 | Mei-Ling, agent |
| 11 | Service-window reporter | `doctor window` | 5/10 | Compares Taipei time to 0–6 AM API window; prints status + seconds-to-window | Brief Reachability §1; agent operator hits it | All, especially agent |

11 transcendence features. All scored ≥5/10 by the rubric. Killed candidates with reasons preserved in `2026-05-09-091741-novel-features-brainstorm.md`.

## Stubs

None. Every row above is shipping scope.

## Known constraints (must be enforced by Phase 3)

- **Rate limit:** Default ≤ 1 req/sec per source, configurable to 5 via `--rate`. Adaptive backoff on 429.
- **TOS guard:** First-run prompt acknowledges non-commercial use intent. Configurable via `--accept-tos` for non-interactive use.
- **Privacy purge:** `查無資料` responses MUST cause local deletion + audit-log entry — non-negotiable per ToS.
- **No-auth posture:** No API key, no user account, no token — pure public-website scraping with browser User-Agent.
- **Per-source rate limiter:** Both `internal/source/fjud/` and `internal/source/fjudkm/` clients MUST use `cliutil.AdaptiveLimiter` and surface `*cliutil.RateLimitError`. Required by AGENTS.md per-source rate-limiting rule.

## Source attribution (for README "Inspired by" / Credits)

- `samttoo22-MewCat/judgement-scrawler` — Python+SeleniumBase scraper; primary inspiration for search-by-court+keyword; we ship a stdlib-HTTP, agent-native, store-backed equivalent.
- `GOV-TW/Judicial-OD` — official open-data spec source; we deliberately don't depend on the API path per user choice but credit the spec.
- `whiskyinsulo/Judicial_Judgements` — open-data parser/cleaner reference.
- `biglawtw/biglaw` — open-law judgment intelligent search; no direct code reuse, philosophical alignment.
- `0xyd/SunnyJudge` — sister project (Sunny Judge API); not directly absorbed but acknowledges the ecosystem.
