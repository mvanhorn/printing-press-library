# bls-pp-cli Absorb Manifest

## Source Tools Surveyed
- **OliverSherouse/bls** (Python, 84★, stale 2020-02): canonical Python wrapper; single-series + multi-series, tidy pandas, key cache at `~/.bls/key`.
- **keberwein/blscrapeR** (R, 117★, active 2026-04): most active wrapper; main fetch, inflation_adjust, qcew_api, niolsa_geo (LAUS state/metro), ships cached datasets.
- **mikeasilva/blsAPI** (R, on CRAN): v2 wrapper, series-list builder by survey.
- **a-finocchiaro/bls-data** (Python, 5★, 2021): per-survey class abstractions (CPI, CES, LAUS).
- **addisonlynch/pyBLS** (Python, 7★, abandoned 2019).
- **lzinga/us-gov-open-data-mcp** (MCP, 98★, active): 40-API multiplexer including a shallow `getBLSData` tool.
- **@leviai/publicfinance-mcp** (npm MCP, bundle of BLS + SEC + Treasury).
- **frasermarlow/tap-bls** (Singer.io tap, 6★, 2025-06): ETL replication.
- **Alex1100/blsjs**, **reaperkrew/bls** (small npm).
- **cridenour/go-bls** (Go, 3★, dead 2014): only prior Go effort.

No feature-rich BLS CLI exists. No BLS-specific MCP server exists.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Fetch single series | OliverSherouse/bls | `series get <id> --start <yr> --end <yr>` | Offline cache, `--json`/`--csv`/`--select`/`--agent`, tidy by default |
| 2 | Batch fetch (up to 50 series per call) | OliverSherouse/bls, blscrapeR | `series batch --ids a,b,c --start <yr>` | Auto-partitions >50 IDs into multiple calls, streams JSON, idempotent |
| 3 | Latest observation | blsAPI | `series latest <id>` | Local cache first; falls through to live |
| 4 | List surveys | blscrapeR | `surveys list` | Stored locally; offline; FTS over abbr + name |
| 5 | Survey detail | blscrapeR | `surveys get <abbr>` | Local cache; includes endpoint coverage |
| 6 | Popular series per survey | blscrapeR | `popular --survey <abbr>` | Cached weekly; offline |
| 7 | Tidy / columnar output | OliverSherouse/bls | `--csv`, `--json`, `--select`, `--agent` | Native CLI output modes; no pandas dep |
| 8 | CPI inflation deflation | blscrapeR `inflation_adjust` | `inflation adjust --amount 100 --from-year 2010 --to-year 2025 [--monthly]` | Local CPI cache → offline; supports CPI-U and CPI-W |
| 9 | Per-survey shortcuts (CPI/CES/LAUS) | a-finocchiaro/bls-data | Sub-resources `cpi`, `ces`, `laus`, `jolts`, `ppi`, `eci`, `productivity`, `qcew`, `oews` | Each survey gets a named subcommand with sensible defaults |
| 10 | Series ID builder | mikeasilva/blsAPI | `series build --survey CPI --area "Los Angeles" --item "All items" --adjust sa` | Joins local areas + items + adjustment tables to synthesize the packed ID |
| 11 | QCEW area+industry aggregator | blscrapeR `qcew_api` | `qcew wages --industry 23 --area "Los Angeles County" --year 2024 --quarter 2` | Local QCEW area/industry tables |
| 12 | Local registration-key cache | OliverSherouse/bls (`~/.bls/key`) | `auth set-token`, `auth status`, env-var precedence | Standard config path |
| 13 | MCP passthrough | lzinga/us-gov-open-data-mcp | Every CLI command auto-mirrored as MCP tool via cobratree | Native MCP server, not a multiplexed proxy |
| 14 | Bulk replication | frasermarlow/tap-bls | `sync` populates local SQLite with series catalog + observations | Generalized; not Singer-shaped |
| 15 | Rate-limit aware retries | implicit in most | `cliutil.AdaptiveLimiter` per-source | Surfaces `*cliutil.RateLimitError`, distinguishes throttling from no-data |

Every absorbed row is shipping scope — no stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Series search (FTS5) | `series search "<query>" [--survey CPI] [--area "Los Angeles"]` | 10/10 | BLS has NO public series-search endpoint. FTS5 over the locally-synced flat-file catalog is the only way to discover a series ID by plain English. The #1 universally cited pain point across every wrapper. |
| 2 | Macro snapshot | `snapshot macro [--csv]` | 9/10 | Curated ~15-series cross-survey batch (`// pp:novel-static-reference`) hitting `POST /timeseries/data/` with `calculations:true`. One command returns CPI, core CPI, headline U3, payrolls, JOLTS openings, PPI, ECI, productivity, etc. with YoY+MoM. |
| 3 | Release calendar | `releases next [--survey CPI] [--within 14d] [--watch]` | 8/10 | BLS publishes the release calendar only as HTML behind Akamai. Local curated table refreshed by sync; supports `--watch` to poll until the next release. |
| 4 | Footnote decoder | `footnotes decode <code...>` | 7/10 | Local `footnotes` table built from BLS `<abbr>.footnote` flat files. The live API returns footnote codes but never their explanations. |
| 5 | Historical extremum | `series extremum <id> [--since 1990]` | 7/10 | SQL over the local `observations` cache: max/min/rank of the latest observation across a configurable window. Devin's "is this the highest since X." |
| 6 | SA/NSA compare | `series compare-sa <stem>` | 6/10 | Decodes position-3 of the packed series ID via local `seasonal_adjustment` lookup; emits both SA and NSA variants side-by-side. |

## Customer model (audit trail)

Personas drawn from research: Mira (macro strategist, NYC), Devin (economics reporter, DC), Sam (RAG/agent engineer), Priya (labor-econ PhD). Full personas in `2026-05-12-131153-novel-features-brainstorm.md`.

## Killed candidates (audit trail)

Series-ID builder (already absorbed), release-watch (folded into `releases next --watch`), inflation adjustment (already absorbed), QCEW cross-area (absorbed), YoY/MoM (flag), surveys stats (no weekly use), macro alias (subsumed), agent-mode resolver (auto-exposed via MCP), release-day diff (extremum is more general), revision tracker (verifiability low). Full table in brainstorm doc.

## Total scope

- **15 absorbed features** (full parity with every existing tool)
- **6 transcendence features** scoring ≥ 6/10
- **0 stubs** — everything is shipping scope
