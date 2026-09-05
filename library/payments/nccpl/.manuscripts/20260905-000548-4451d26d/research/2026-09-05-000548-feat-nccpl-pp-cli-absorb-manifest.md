# NCCPL CLI — Absorb Manifest

Ecosystem finding: **no CLI, no MCP server, no Claude plugin, and no library wrapper exists for
NCCPL.** `crowd-sniff` returned `exit=4, no endpoints discovered`; `gh repo search nccpl` returned
nothing; npm returned nothing. The only public code touching this API is two Playwright probe
scripts in `hmehmood56-debug/PSX-Trader`. Every incumbent (Portfolio360, FinHisaab, Youngs
Capital, BullsView, StockIntel) is a web dashboard that renders a number and discards it.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Daily FIPI net flow headline | Portfolio360 / FinHisaab | (generated endpoint) fipi data | offline, `--json`, arbitrary range, DD/MM/YYYY encoded correctly |
| 2 | LIPI net flow by investor class | Portfolio360 | (generated endpoint) lipi data | all 10 investor codes, not just the headline |
| 3 | FIPI sector-wise flows | NCCPL site | (generated endpoint) fipi-sector data | range query, offline |
| 4 | LIPI sector-wise flows | NCCPL site | (generated endpoint) lipi-sector data | range query, offline |
| 5 | FIPI normal (volume+value by market type) | NCCPL site | (generated endpoint) fipi-normal data | `records` envelope handled |
| 6 | LIPI normal (volume+value by market type) | NCCPL site | (generated endpoint) lipi-normal data | `data` envelope handled — asymmetric to #5 |
| 7 | Market trade value/volume series | NCCPL graph-data | (generated endpoint) market latest | arbitrary window via `--limit` |
| 8 | Market series by explicit range | NCCPL graph-data | (generated endpoint) market range | multipart encoding handled |
| 9 | MTS open positions per symbol | NCCPL site | (generated endpoint) mts data | offline, SQL-composable |
| 10 | MTS financiers/financees counts | NCCPL site | (generated endpoint) mts-financiers data | offline |
| 11 | MTS force release | NCCPL site | (generated endpoint) mts-force-release data | offline |
| 12 | MTS top-15 financiers | NCCPL site | (generated endpoint) mts-top-financiers data | offline |
| 13 | MTS amount refinanced | NCCPL site | (generated endpoint) mts-refinanced data | offline |
| 14 | MFS open positions per symbol | NCCPL site | (generated endpoint) mfs data | offline; carries free-float % |
| 15 | MFS top-15 financees/financiers | NCCPL site | (generated endpoint) mfs-top data | offline |
| 16 | MSF open positions per symbol | NCCPL site | (generated endpoint) msf data | offline |
| 17 | MSF top-15 buyer/seller pairs | NCCPL site | (generated endpoint) msf-top data | offline |
| 18 | SLB open positions per symbol | NCCPL site | (generated endpoint) slb data | offline; the short-interest proxy |
| 19 | VAR margins / haircut / free float per symbol | NCCPL site | (generated endpoint) var-margins data | offline; the only public free-float source |
| 20 | Settlement UIN-wise | NCCPL site | (generated endpoint) settlement-uin data | offline |
| 21 | Settlement CM-wise | NCCPL site | (generated endpoint) settlement-cm data | offline |
| 22 | Unlisted TFC transactions | NCCPL site | (generated endpoint) tfc data | offline |
| 23 | Per-resource latest publication date | NCCPL site | (generated endpoint) fipi latest-date (+19 siblings) | all 21 probed; no CSRF needed |
| 24 | Sector ranking by absolute net flow | Portfolio360 / PSX-Trader | (behavior in nccpl-pp-cli panel) `--sort` | offline, any window |
| 25 | USD and PKR flow views | Portfolio360 | (behavior in nccpl-pp-cli panel) `--currency` | `net_value_USD` is native |
| 26 | Window aggregation (week / 20-session / YTD) | Portfolio360 | (behavior in nccpl-pp-cli panel) `--from/--to` | arbitrary window, not fixed presets |
| 27 | Sector x investor-type net-flow matrix | Portfolio360 | (behavior in nccpl-pp-cli panel) `--resource fipi-sector --pivot` | the headline incumbent view, offline and SQL-composable |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Arithmetic invariant validator | `verify` | hand-code | NCCPL publishes two exact identities for free — FIPI net = -LIPI net, and every sector row nets to zero across the 10 investor classes. Neither is checkable without holding both sides locally. A date that fails is corrupt input that must never reach a regression. | Use this command to prove a date's numbers are internally consistent before they reach a regression — it answers "are these values correct." Do NOT use it to find out which dates are absent from the local store; a date that was never fetched cannot fail an invariant. Use 'coverage' for missing dates instead. |
| 2 | Per-resource coverage and gap ledger | `coverage` | hand-code | `latest-date` reports one date per resource and is structurally blind to holes in the middle of the archive. Only a local date-set diff can see a missing session. `--exit-code` makes it a pipeline pre-flight, not a report. | Use this command to find out which dates are missing, which resources are stale, and how wide each date's data actually is. Do NOT use it to check whether stored numbers are arithmetically consistent; use 'verify' instead. Do NOT use it to test whether the live API still answers correctly; use 'contract-check' instead. |
| 3 | Vintage-stamped research panel export | `panel` | hand-code | NCCPL exposes no publication timestamp, so ex-ante availability can only be established by stamping `observed_at` at sync time — it is not reconstructible afterwards. Emits long-format `(date, key, metric, value, observed_at)` and never interpolates a gap. | Use this command to export stored NCCPL data as a research panel for regression or ingestion into another store. Do NOT use it to build a per-symbol leverage cross-section; use 'leverage' instead. Do NOT use it to find which symbols were listed on a given date; use 'universe' instead. |
| 4 | Point-in-time symbol roster | `universe` | hand-code | NCCPL only publishes risk parameters and settlement records for symbols that were actually eligible that day, so presence/absence across dates yields a clearing-house-derived liveness signal. It can disagree with a price-staleness filter for reasons a price-staleness filter cannot move for — which is the definition of a control. | Use this command to reconstruct which symbols existed and were eligible on a past date, and to find when a symbol stopped appearing. Do NOT use it to read a symbol's actual risk parameters or leverage figures; use 'panel --resource var-margins' or 'leverage' instead. |
| 5 | Unified leverage and short-interest panel | `leverage` | hand-code | Four endpoints (MTS, MFS, MSF open positions plus SLB) across three different response envelopes, all keyed on the same `(date, symbol)`. No API call performs this join, and no incumbent exposes symbol-level leverage for cross-sectional work. | Use this command for the cross-market leverage and short-interest view of one or many symbols. Do NOT use it for VAR margins, haircuts or free float; use 'panel --resource var-margins' or 'risk-changes' instead. Do NOT use it for investor-class capital flows; use 'panel' instead. |
| 6 | Risk-parameter change detector | `risk-changes` | hand-code | A single-date endpoint cannot express a change. Diffing consecutive stored `var-margins` rows dates every free-float, VAR and haircut step — the corporate actions, lockup expiries and clearing-house re-ratings that exist nowhere else in this market. | Use this command to find when a symbol's free float, VAR margin or haircut stepped, and by how much. Do NOT use it to read the level of those fields on a date, or to build a cap-weighting input; use 'panel --resource var-margins' instead. |
| 7 | Live endpoint self-test | `contract-check` | hand-code | Asserts, per date-format family and per response-envelope key, that each `/data` POST returns a non-empty array for a date its own `latest-date` just reported. That date must by construction have data, so a zero-row response is unambiguously a defect — catching auth expiry, date-format regressions and envelope drift the day they happen. | Use this command to test whether the live API, the session handshake and the request encoding are still working. Do NOT use it to check the local store's completeness or freshness; use 'coverage' instead. |

**Stubs:** none. Every row above is shipping scope.

**Renamed from brainstorm:** C7 `doctor` -> `contract-check`. `doctor` is a reserved generated
framework command and would have been shadowed.

**Buildability totals:** 7 of 7 transcendence rows are `hand-code`; 0 are `spec-emits`.
