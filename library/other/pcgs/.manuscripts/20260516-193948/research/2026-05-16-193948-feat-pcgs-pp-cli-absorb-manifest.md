# PCGS CLI — Absorb Manifest

## Source tools surveyed (Step 1.5a)

No community CLI, SDK, MCP server, or Claude plugin wraps the PCGS Public API.
GitHub code search surfaces 11 consumer applications that embed PCGS API calls
inline (BobdaFett/pcgs-inv, BobdaFett/pcgs-inv-gui, Cfomodz/what-bot,
Cfomodz/PCGS-slab-picture-to-listing-tool, pixiitech/lustre, lbruton/StakTrakr,
ammonfife/sakima.co, Arunavanag1/anags-bullion-tracker, PhillyG76/AuctionEye,
prescia-ai/Prescia-Coin, evansminotwood/Aureus, chadstachowicz/coin_ml,
MEVERIK-SOLUTION/mince) — none are general-purpose wrappers. npm and PyPI have
zero packages. There is no first-party PCGS SDK.

Recurring inline-usage patterns observed across the consumer apps:
- One-call cert-to-metadata lookup (every app).
- Naive multi-cert batch loops (every batch-shaped app).
- No quota awareness or local persistence; transient errors silently drop rows.
- No separation of immutable vs mutable PCGS fields; cert-identity fields can
  be corrupted by a transient bad response.

The absorb manifest below treats the **PCGS API surface itself** as the
incumbent to match.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | Lookup CoinFacts by cert | `GET /coindetail/GetCoinFactsByCertNo/{certNo}` | `coin facts cert <cert> [--retrieve-all-data] [--no-cache] [--max-age 7d]` | `--json`/`--select`/`--csv`, durable local cache, typed exit codes (0 success, 3 not found, 4 invalid input, 7 rate-limit), surfaces `IsValidRequest`/`ServerMessage` cleanly | spec-emits |
| 2 | Lookup CoinFacts by PCGSNo + grade | `GET /coindetail/GetCoinFactsByGrade` | `coin facts grade --pcgs-no <n> --grade <g> [--plus]` | structured flags vs raw query params; cache + `--select` | spec-emits |
| 3 | Lookup CoinFacts by holder barcode | `GET /coindetail/GetCoinFactsByBarcode` | `coin facts barcode <barcode> --service pcgs` | barcode workflows (mobile scan, then enrich) | spec-emits |
| 4 | Auction Prices Realized by cert | `GET /coindetail/GetAPRByCertNo/{CertNo}` | `coin apr cert <cert>` | CSV-friendly output for spreadsheet import; cache | spec-emits |
| 5 | APR by PCGSNo + grade with date range | `GET /coindetail/GetAPRByGrade` | `coin apr grade --pcgs-no <n> --grade <g> [--plus] [--start --end --limit]` | date filtering, limit, table view; cache | spec-emits |
| 6 | APR by barcode | `GET /coindetail/GetAPRByBarcode` | `coin apr barcode <barcode> --service pcgs [--start --end]` | parity with cert variant | spec-emits |
| 7 | Images by cert | `GET /coindetail/GetImagesByCertNo` | `coin images <cert>` | JSON array of URLs; cache | spec-emits |
| 8 | Banknote facts by cert | `GET /banknotedetail/GetBanknoteByCertNo` | `banknote facts cert <cert> [--lang en\|fr\|...]` | language flag; cache | spec-emits |
| 9 | Banknote by PCGSNo + grade | `GET /banknotedetail/GetBanknoteByGrade` | `banknote facts grade --pcgs-no <n> --grade <g>` | structured flags | spec-emits |
| 10 | Banknote images | `GET /banknotedetail/GetBanknoteImagesByCertNo` | `banknote images <cert>` | parity with coin | spec-emits |
| 11 | Orders by submission no | `GET /orderdetail/GetOrdersBySubmissionNo` | `order submission <submission-no>` | submitter view; cache | spec-emits |
| 12 | Orders by date range | `GET /orderdetail/GetOrdersByDateRange` | `order range --start <d> --end <d> [--page --page-size]` | pagination; cache | spec-emits |
| 13 | Naive multi-cert batch lookup | Consumer-app convention (BobdaFett/pcgs-inv-gui, Arunavanag1/anags-bullion-tracker, Cfomodz/PCGS-slab-picture-to-listing-tool roll their own loops) | `coin batch [--file - --format auto]` (resumable/dry-run variants are transcendence) | parses CSV/JSON wrappers/JSONL/plain text fixtures; structured progress; cache hits skipped | hand-code |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Quota status & forecast | `quota [--for <file>]` | 10/10 | hand-code | Reads `lookup_log` for today's UTC window, computes used/remaining/reset; with `--for`, parses the fixture, dedupes against cache, and projects required call count + warn/block thresholds | VISION.md mandates 1000/day hard cap with quota command + 80%/95% soft warnings; brief Top Workflow 4 |
| 2 | Dry-run cost estimator | `coin batch --file <f> --dry-run` | 10/10 | hand-code | Same fixture parser as `coin ingest` + cache-hit lookup against local `coin` table; emits "N live calls, M cache hits, would consume X% of remaining quota" without spending a single call | VISION.md 1000/day constraint + brief Top Workflow 2 |
| 3 | Mutable-only sync | `sync [--cert … \| --all] [--max-age 7d]` | 10/10 | hand-code | For each in-scope cached row, re-calls `GetCoinFactsByCertNo` (and APR/Images if requested), updates ONLY Population/PopHigher/PriceGuideValue/AuctionList/Images/CoinFactsNotes columns, never touches identity fields (CertNo, PCGSNo, Name, Year, Mintage, MintMark, MintLocation, MetalContent, Diameter, Edge, Weight, Country, Designer, Grade, Designation, Series, Category, Variety), emits per-field diff | VISION.md "Mutable-only sync" rule + brief Top Workflow 3 |
| 4 | Spec-number population sweep | `coin pop-curve <pcgs-no> [--plus] [--include-details] [--grades 1-70]` | 9/10 | hand-code | Iterates grades 1-70 (+Plus when `--plus`; extends to 82-98 Details codes when `--include-details`) calling `GetCoinFactsByGrade`; writes `(PCGSNo, GradeNo, PlusGrade) → Population/PopHigher` rows to local store and prints the full pop curve as a scarcity table | brief Top Workflow 5; PCGS-specific 1-70 + Plus + Details (82-98) grade axes; articles: tips-for-utilizing-the-pcgs-population-report, the-joys-of-collecting-low-ball-morgan-dollars, details-grade-pcgs-coins-represent-affordable-opportunities-for-collectors |
| 5 | Cert-list ingest from VISION fixtures | `coin ingest <path> [--format auto]` | 10/10 | hand-code | Auto-detects CSV (with/without header, multi-column) / JSON wrappers (`{slabs:[…]}` and `{holdings:[…]}`) / JSONL / plain text (with/without comments); normalizes plus-grade slab IDs like `7130.67/51225377`; emits a clean cert list to stdout that `coin batch` and `sync` can consume | brief: "Local fixtures (`test-input-generated/`) are first-class" — 11 valid + 7 deliberately-bad fixture shapes already exist on disk |
| 6 | Lookup-log audit | `audit [--since 7d] [--failed] [--endpoint …] [--by-day] [--cert <c>]` | 8/10 | hand-code | Pure SQL over the `lookup_log` table; supports `--failed` (IsValidRequest=false rows), `--by-endpoint`, `--by-day` aggregates, `--cert <c>` per-cert history | brief Data Layer defines `lookup_log`; agent-shaped output requirement from VISION |
| 7 | Submission fan-out | `order hydrate <submission-no> [--with-images]` | 7/10 | hand-code | Calls `GetOrdersBySubmissionNo`, extracts every CertNo, fans out per-cert `GetCoinFactsByCertNo` (and `GetImagesByCertNo` when `--with-images`), respects cache, refuses to start when remaining quota < cert count | submitter persona's manual fan-out frustration; brief Top Workflow chaining |
| 8 | Local search across cached coins | `search --text <q> [--year …] [--min-pop …] [--max-price …]` | 9/10 | hand-code | FTS5 over Name/Country/SeriesName/Category/Designer/MintLocation/MajorVariety/MinorVariety/DieVariety + range filters on Year/Grade/PriceGuideValue/Population/PopHigher/Mintage/Weight/Diameter; **no API call** | brief Data Layer explicitly lists FTS + range columns |
| 9 | Resumable cache-warm | `coin batch --file <f> --resumable [--checkpoint <path>]` | 9/10 | hand-code | Extends absorbed `coin batch` with a checkpoint file recording last-completed cert and per-cert outcome; on resume, skips completed certs and stops cleanly when remaining quota < remaining work (splits a 1000-row CSV across multiple UTC days) | VISION 1000/day hard cap + `pcgs-coin-list.csv` as first-class input |
| 10 | Stale-field watcher | `stale [--field price-guide\|population\|auction\|images\|notes] [--older 30d]` | 8/10 | hand-code | Local SQL over `coin` + `lookup_log`: list cached coins whose specified mutable field hasn't been refreshed in N days; drives the `sync` queue. **No API call.** | article: keeping-the-pcgs-price-guide-updated — PCGS only refreshes Price Guide "eventually" for 310k+ priced items; user-side question is which of MY coins are stale |
| 11 | Pop-1 inventory | `coin pop1 [--year <n>] [--denom <d>] [--min-grade <g>]` | 7/10 | hand-code | Local query over cached `coin` rows where `Population == 1`; optional filters on Year, Denomination, min-grade. **No API call.** | article: tips-for-utilizing-the-pcgs-population-report — "Pop 1" is the apex-rarity signal that the article repeatedly emphasizes drives auction value |

## Final surface (post-user-review reduction)

The 11 transcendence rows above describe capabilities. The user requested a
slimmer command surface; here is the locked Cobra/MCP tree.

**6 novel Cobra commands** (all `hand-code`):

| Command | Subsumes |
|---|---|
| `coin batch [--file PATH] [--list-certs] [--dry-run] [--resumable --checkpoint PATH] [--no-passthrough] [--key-column NAME]` | rows 2, 5, 9 (dry-run estimator, fixture ingest, resumable cache-warm). Per-row pass-through: every non-cert column in the input is round-tripped to output as `_keep.<col>`. Optional `--key-column` promotes one column to a top-level `_key` for ergonomic spreadsheet round-trips. |
| `sync [--all \| --cert CERT] [--field FIELD] [--older Nd] [--dry-run]` | rows 3, 10 (mutable-only sync, stale-watcher). `--dry-run --older 30d --field price-guide` IS the old `stale` command. |
| `coin pop-curve <pcgs-no> [--plus] [--include-details] [--grades 1-70]` | row 4. |
| `audit [--since Nd] [--failed] [--endpoint X] [--by-day] [--by-endpoint] [--cert C]` | row 6. |
| `order hydrate <submission-no> [--with-images] [--dry-run]` | row 7. |
| `search [--text Q] [--year N] [--denom D] [--series S] [--min-pop --max-pop --top-pct] [--min-grade --max-grade]` | rows 8, 11 (local search, pop1). `--max-pop 1` IS the old `coin pop1`. `--top-pct P` is the new continuous-rarity flag (top P% by ascending Population across the scoped cohort). |

**Universal quota mechanics (no dedicated `quota` command):**
- Stderr quota line on every API-calling command (suppressed by `--quiet`).
- Global `--quota` root flag: prints today's used/remaining/reset and exits, zero API calls.
- Global `--quota-only` root flag: same as `--quota` but JSON to stdout for agents.

**Hand-code commitment: 8 features.** The 6 commands above + a base `coin batch` parsing/runner + a `cliutil.StderrQuota` helper consumed by every API-calling command.

## Resource/Cobra/MCP shape

Internal YAML resource grouping is `coin` / `banknote` / `order` (the `/coindetail/`, `/banknotedetail/`, `/orderdetail/` URL prefixes are kept in the spec's endpoint paths but stripped from the resource names).

| Cobra path | MCP tool |
|---|---|
| `pcgs coin facts-cert <cert>` | `pcgs_coin_facts_cert` |
| `pcgs coin facts-grade --pcgs-no --grade [--plus]` | `pcgs_coin_facts_grade` |
| `pcgs coin facts-barcode <bc> --service` | `pcgs_coin_facts_barcode` |
| `pcgs coin apr-cert <cert>` | `pcgs_coin_apr_cert` |
| `pcgs coin apr-grade --pcgs-no --grade [--start --end --limit]` | `pcgs_coin_apr_grade` |
| `pcgs coin apr-barcode <bc> --service [--start --end]` | `pcgs_coin_apr_barcode` |
| `pcgs coin images <cert>` | `pcgs_coin_images` |
| `pcgs coin batch ...` | `pcgs_coin_batch` |
| `pcgs coin pop-curve <pcgs-no> ...` | `pcgs_coin_pop_curve` |
| `pcgs banknote facts-cert <cert> [--lang]` | `pcgs_banknote_facts_cert` |
| `pcgs banknote facts-grade --pcgs-no --grade` | `pcgs_banknote_facts_grade` |
| `pcgs banknote images <cert>` | `pcgs_banknote_images` |
| `pcgs order submission <sub-no>` | `pcgs_order_submission` |
| `pcgs order range --start --end [--page --page-size]` | `pcgs_order_range` |
| `pcgs order hydrate <sub-no> [--with-images]` | `pcgs_order_hydrate` |
| `pcgs sync ...` | `pcgs_sync` |
| `pcgs audit ...` | `pcgs_audit` |
| `pcgs search ...` | `pcgs_search` |
| `pcgs doctor` | `pcgs_doctor` |
| `pcgs sql ...` | `pcgs_sql` |

~20 MCP tools — clean, brief, no `detail` cruft.

## Compose-with-other-data recipes (documentation only, no code)

These ship as a `## Compose with other data` README section and as recipes in
SKILL.md. The CLI does not fetch any of the external sources — these are
patterns the user (or an agent) follows to pair PCGS data with another
source. All 10 are agent-discoverable in SKILL.md.

| # | If you also have | Use case | Pattern |
|---|------------------|----------|---------|
| R1 | Spot bullion prices (gold, silver, Pt, Pd) | Bullion-floor analysis: spot when grade premium has collapsed to melt value | Pipe `coin facts cert --json` MetalContent + Weight through `jq` and multiply by spot |
| R2 | Heritage / Stacks-Bowers / GreatCollections lot data | Cross-house spread analysis; find lots under PCGS APR | Join `coin apr cert --json` with auction-house APIs by date + lot |
| R3 | US Mint mintage records | Survival-rate scarcity (PCGS pop ÷ mintage) — more honest than raw pop | Cross-walk `coin pop-curve --json` PCGSNo to mintage table |
| R4 | NGC Population Report | True-market scarcity (PCGS pop + NGC pop) | Join `coin pop-curve --json` with NGC pop snapshot on (Year, Denom, Mint, Grade) |
| R5 | eBay completed-sale listings | Retail-vs-auction spread; counterfeit-cheap alerts | Pair `coin apr cert --json` per-result with eBay's completed-sales search |
| R6 | FRED CPI / BLS inflation data | Inflation-adjusted realized prices | Apply CPI ratio to `coin apr grade --json` Date + Price fields |
| R7 | PCGS Set Registry data | "Which of my coins would join a top set?"; cheapest path to top-100 | Cross-walk `search --json` cached collection vs Set Registry slots |
| R8 | PCGS show calendar / submission schedule | Pre-show inventory refresh planning | Filter `stale --json` output to coins in series you're bringing to a show |
| R9 | Holder-generation catalog (OGH, rattler, blue label, gold-shield) | Holder-type premium analysis | Tag cached records by holder-gen pattern matched on cert range + image |
| R10 | Personal collection system (downstream) | Diff PCGS-truth vs your collection's cert list; mismatch detection | `coin ingest` → diff against your downstream collection's cert export |

## Killed candidates (audit trail)

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| verify-identity | Thin wrapper + assert; identity-drift detection is better expressed as a flag on `sync` | sync |
| pop-shift | Useless on first print (no prior pop-curve snapshots to diff) | coin pop-curve |
| coin apr-window | Borderline wrapper; ship local count/min/max/median as `--stats` on `coin apr grade` | coin apr grade |
| coin melt | Niche; spot prices have to be passed by flag, which makes it a calculator | (none) |
| quota probe | One-shot diagnostic, not a weekly command | audit |
| order images | Folds into `order hydrate --with-images` | order hydrate |
