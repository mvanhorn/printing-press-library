# PBS CLI — Absorb Manifest

## Ecosystem search result: THE FIELD IS EMPTY
Searched (2026-09-08): GitHub CLIs, MCP servers, Claude plugins/skills, npm, PyPI, and the
public printing-press registry (504 entries / 22 categories, enumerated from the repo).
**There is no competing tool of any kind for PBS price data.** The only PBS repo on GitHub is
`cerp-analytics/pbs2017`, which is digitised 2017 CENSUS data, not prices. No npm package, no
PyPI package, no MCP server. Registry: zero Pakistani price coverage; `other/fred` and
`other/us-data` are US-only. So the absorbed baseline is set by what an analyst does BY HAND
plus the framework commands the generator emits.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Locate the release files | Manual: scroll /price-statistics/ | `(generated endpoint) index get` | Extracts the file-link list from the served HTML in one call |
| 2 | Read one release file | Manual: click, download, open Excel | `(generated endpoint) file get` | Fetch any annexure/report/CPI annex by exact filename |
| 3 | Enumerate releases as a dated, sorted series | Manual: eyeball the page | `pbs-pp-cli releases` | Parses BOTH JS literal arrays, sorts by parsed date (the array is NOT sorted), reconciles 35 filename shapes, flags the 2 rows whose filename date disagrees with the index date |
| 4 | Build the local panel | Manual: retype from Excel/PDF | `pbs-pp-cli sync` | Resumable, per-release checkpointed, 3-state coverage accounting, fetches BOTH files per release |
| 5 | Full-text search over items | none | `(behavior in pbs-pp-cli search)` | Framework FTS5 over item descriptions, offline |
| 6 | Raw SQL over the panel | none | `(behavior in pbs-pp-cli sql)` | Framework read-only SQL — also how the killed `quintile` gap series is reached |
| 7 | Health / reachability | Manual: open the site | `(behavior in pbs-pp-cli doctor)` | Framework, probing /robots.txt (text/plain) to dodge the HTML-200 doctor bug |
| 8 | Agent-native output | Manual: retype | `(behavior in pbs-pp-cli --json/--agent/--csv/--select/--compact)` | Framework, on every command |

Rows 3 and 4 are Build Priority 1 infrastructure, hand-written. The subagent scored a standalone
`releases` command 8/10 but correctly classified it as plumbing, not transcendence: `sync` cannot
enumerate without it.

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|-------------------------|------------------|
| 1 | Cross-city price spread + dispersion trend | `spread` | hand-code | 10 | Joins price across all urban centres for one item-week out of the accrued panel; the dispersion series and city rank stability need every release held at once. Nobody publishes a Pakistani sub-national price-spread series. Prints contributing-city N and the zero/blank/N.A. cells excluded; refuses below `--min-cities`. | Use this command to compare one item ACROSS CITIES in a single week, or to trend that dispersion with `--series`. Do NOT use it to follow one item's level over weeks for named cities; use 'drift' instead. Do NOT use it to rank many items in one week; use 'movers' instead. |
| 2 | Single-item multi-city series | `drift` | hand-code | 10 | Selects price rows by item DESCRIPTION — the only stable join key, since `Sr.` restarts at 1 in each of three weekly re-ranked sections. The brief's workflow #2 ("wheat flour in Quetta vs Karachi since 2023") is impossible today because each week is a standalone file. Missing weeks emit a row carrying value_state, never an interpolated value. | Use this command to follow ONE item's price over TIME for named cities. Do NOT use it for the cross-city distribution in one week; use 'spread' instead. Do NOT use it to find which items moved; use 'movers' instead. |
| 3 | Custom re-weighted basket | `basket` | hand-code | 10 | Joins stored weight rows (PBS republishes the item weight vector in EVERY release) against price levels to rebuild an SPI-style index over any item subset, per city. Cross-city figures are labelled explicitly unweighted because PBS's CITY weight vector is not published. | Use this command to compute an index over a chosen item subset. Do NOT use it to inspect or diff the weight vector; use 'weights' instead. It does not attempt to reproduce the published National Ave. — see 'verify' for why that reconciliation is expected to diverge. |
| 4 | Weight-vector inspection and drift | `weights` | hand-code | 8 | The vector is republished weekly with NO upstream change log, so any re-weighting is invisible to anyone who did not store the prior week — the accrued store is the only place a diff exists. `--check-total` asserts the TOTAL rows sum to exactly 100.0000 on both the lowest-quintile and combined columns. | Use this command to read or diff the item weight vector and check its totals. Do NOT use it to compute a price index from those weights; use 'basket' instead. |
| 5 | Extraction verification | `verify` | hand-code | 10 | Audits the parser FROM OUTSIDE the parser against measured invariants: weight TOTALs = 100.0000; the three section counts (increased/decreased/unchanged) sum to the release's item count; MIN <= AVG <= MAX per cell; all urban centres resolved across the three stacked row-blocks whose third block has irregular 2-col and 4-col merges; `% Change over` confirmed 0.0 so percentages come from levels; and recomputed unweighted national average vs published `National Ave.`, reported as an EXPECTED ~0.45% median divergence because city weights are unpublished. Demonstrates the null-coercion cost live (Rice IRRI-6/9: 154.00 correct vs 128.36, -16.6%, if zeros are read as prices). | Use this command to check whether parsed CELL VALUES are internally consistent for releases already stored. Do NOT use it to find missing releases; use 'coverage' instead. Do NOT use it to detect upstream changes; use 'revisions' instead. A national-average divergence is the expected result, not a failure. |
| 6 | Coverage and gap map | `coverage` | hand-code | 10 | Reconciles the scraped index against the store and against a TOLERANCE-BASED weekly cadence — deliberately not a weekday rule, because releases fall on six different weekdays and an "expected Thursday" detector emits 18 false gaps. Reports index coverage, the 7-week hole 2024-09-12..2024-11-07, the xlsx-vs-PDF-only split, and per-release counts of numeric-0 / blank / N.A. / real numerics as THREE DISTINCT STATES under `--state-census`. | Use this command to answer what is present and what is MISSING, at release and cell-state granularity. Do NOT use it to check whether present values parsed correctly; use 'verify' instead. Gaps are reported, never filled. |
| 7 | Upstream revision and deletion detector | `revisions` | hand-code | 10 | Compares stored sha256 and stored index rows against the live site. Only meaningful because the store accrues what PBS overwrites: Wayback's earliest SPI capture is 2025-10-22 while the live index reaches 2023-07-13; PBS already 404'd its whole pre-2023 tree; and one CPI annex URL is listed under both March 2024 and February 2024, so February 2024 is already lost. Catches a held release that now 404s, bytes changed under a stable filename, and an index row re-pointed to a different filename or date. | Use this command to detect what PBS has CHANGED, REWRITTEN or DELETED upstream since capture. Do NOT use it to audit parse correctness; use 'verify' instead. Do NOT use it to fetch new releases; use 'sync' instead. |
| 8 | Top movers, derived from levels | `movers` | hand-code | 9 | Recomputes every item's percent change from stored LEVELS across adjacent releases, because the published percent and impact columns are measured to be genuinely 0.0 while levels are correct — the ranking does not exist in the source file and cannot be read out of it. Per-city rankings across all urban centres are a panel query, not a file read. Tukey fences FLAG outliers; nothing is dropped or smoothed. | Use this command to rank MANY items within ONE release by percent change, nationally or per city. Do NOT use it to follow a single item over time; use 'drift' instead. Do NOT use it to compare one item across cities; use 'spread' instead. |

**Hand-code count: 8 of 8.** The spec emits only two endpoint commands (`index get`, `file get`);
every panel feature is hand-written. Plus two Priority-1 hand-written infrastructure commands
(`releases`, `sync`) and the shared parsers. No stubs. Nothing in this manifest ships as a
placeholder.

**Design invariants binding on all eight:** every command prints the observation count it
computed over plus the excluded zero/blank/N.A. counts — the three states are never coerced
together; item description is the only join key, never `Sr.`; no command fills, smooths,
estimates or back-dates a missing observation; outliers are flagged with Tukey fences, never
dropped.

**CORRECTION the subagent inherited from me and the build must apply:** the weight vector and the
TOTAL rows are in the SPI **REPORT** file (`report`/`reportExcel`), NOT the annexure — `TOTAL`
occurs zero times in annexure text. So `weights`, `basket --weight`, and
`verify --check total-weights` all depend on the report parser, and `sync` must fetch BOTH files
per release. Also: only ONE `TOTAL` shared string exists; a shared string is stored once and
referenced N times, so string frequency is never a row count.

## Killed (audit trail)
| Candidate | Reason |
|---|---|
| `converge` | Merged into `spread --series --rank-stability`. Two commands taking the same `<item>` and both reporting cross-city dispersion is exactly the sibling overlap that makes an agent pick wrong; rank-stability was the only addition. |
| `nulls` | Merged into `coverage --state-census` (release-level state accounting is what coverage already is). The -16.6% coercion demonstration moved into `verify`. |
| `wage` (8/10) | Real Appendix-B novelty and the strongest deferred idea (real wage in food units), but Appendix-B covers a DIFFERENT and smaller city set, so it needs a second city-resolution path — a second parser, not a second command. Scope creep for a first print already carrying the stacked-row-block parser plus the PDF path for most releases. Revisit at reprint. |
| `inputs` (8/10) | Same second-parser dependency, plus its own unit trap: CNG is priced per litre in Punjab and per kg elsewhere, needing a unit-aware aggregation guard that refuses to average across mixed units. Deferred with `wage`. |
| `crosswalk` (8/10) | VERIFIABILITY failure. The description-match width between the weekly SPI items and the monthly CPI-annex items has not been measured, so the join could be 5 items wide or 40 with no way to confirm at generation time. Measure the overlap first, then ship. |
| `quintile` (9/10) | Highest-scoring cut and a close call. The Q1-Q5 index with WoW/YoY is published as-is in every release, so only the Q1-minus-Q5 gap series is novel — and once spi_index rows are stored, that gap is one expression away through the framework `sql` command. Does not earn a Cobra command in a first print. |
| `releases` (8/10) | Not transcendence — Build Priority 1 infrastructure. Ships regardless (row 3 above) because `sync` cannot enumerate without it. |
| `provincial` | REJECTED UNDER THE NO-FABRICATION RULE. A provincial aggregate is a weighted mean of cities, and PBS's city weight vector is not published in these files. An unweighted provincial mean would look official and be wrong. |
| `realwage` | Folded into `wage --in-terms-of`, then deferred with it. Standalone would have duplicated the Appendix-A/B city-set intersection logic under a second name. |
