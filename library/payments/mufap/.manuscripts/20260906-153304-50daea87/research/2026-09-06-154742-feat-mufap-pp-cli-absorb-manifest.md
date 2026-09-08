# MUFAP CLI — Absorb Manifest

## Prior-art search result: NONE
No MUFAP CLI, MCP server, npm/PyPI wrapper, or Claude skill exists. Searched GitHub, npm,
PyPI, and the MCP directories. The incumbent is **the MUFAP website itself**, so the
"absorbed" set below is the site's own capability surface: everything a human can do on
mufap.com.pk must be reachable from this CLI.

## Absorbed — match every capability the site offers
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Daily NAV + return table | IndustryStatDaily tab=1/2 | (behavior in mufap-pp-cli panel) stored daily panel | 21y queryable, offline, --json/--select |
| 2 | Offer / repurchase / load schedule | IndustryStatDaily tab=3 | (behavior in mufap-pp-cli panel) `--tab pricing` | joins to the same (fund,date) key |
| 3 | Payout / ex-NAV | IndustryStatDaily tab=4 | (behavior in mufap-pp-cli panel) `--tab payout` | dated, diffable |
| 4 | Total expense ratio | IndustryStatDaily tab=5 | (behavior in mufap-pp-cli panel) `--tab ter` | cross-fund comparison |
| 5 | Monthly net assets per fund | IndustryStatMonthly | (behavior in mufap-pp-cli exposure) | PKR amounts, not just display |
| 6 | Monthly net sales / redemptions | WebMonthlyNetSales | (behavior in mufap-pp-cli panel) `--tab netsales` | category flow series |
| 7 | AMC directory | /AMC/GetAMCList | (generated endpoint) amcs list | typed, --json |
| 8 | Fund directory by AMC | /TopHolding/GetFundNameByAMC | (generated endpoint) funds by-amc | typed, --json |
| 9 | Per-fund asset allocation | /Industry/GetAssetAllDetailbyId | (generated endpoint) allocation get | amounts + derived percent |
| 10 | Unit-holder pattern | /Industry/GetUnitPatternHolder | (generated endpoint) unitholders get | typed, --json |
| 11 | Reporting-period list | /Industry/GetDateList | (generated endpoint) dates list | drives backfill planning |
| 12 | Payout announcements | /WebPost/GetAnnouncementPayoutData | (generated endpoint) payouts list | typed, --json |
| 13 | VPS age / cash / withdrawal breakups | VoluntarilyPensionSch/*Json | (generated endpoint) vps age-wise / retired-cash / withdrawals | typed, --json |

## Transcendence — only possible with a local dated panel
| # | Feature | Command | Buildability | Why only we can do this |
|---|---------|---------|--------------|-------------------------|
| 1 | Market-implied PKR short rate | rates | hand-code | Cross-sectional median of every MM fund's yield per date; needs the whole panel local. Fills psx-research `rates.py`, empty because SBP is Cloudflare-blocked. |
| 2 | Industry equity exposure (PKR) | exposure | hand-code | Fan-out per-fund allocation across all AMCs, summing the AMOUNT column because percent is 0 pre-2024. Asset-side counterpart to NCCPL MUTUAL FUNDS net flow. |
| 3 | Dated panel backfill | backfill | hand-code | Every path is date-driven; generator emits no resource mirror. Stamps observed_at. |
| 4 | Panel query | panel | hand-code | Site renders one date at a time; this queries across dates. |
| 5 | Coverage ledger | coverage | hand-code | Records a date even at zero rows — separates fetched-and-empty from never-fetched. |
| 6 | Arithmetic invariant check | verify | hand-code | sum(asset %) − liabilities% == 100; site's own "100%" label is hardcoded and hides failures. |
| 7 | Universe width | universe | hand-code | Width moves 17 → 526 funds; a silently narrowing universe fakes verdicts. |
| 8 | Return dispersion | dispersion | hand-code | Needs every fund's return for a date in one table. |
| 9 | Validity freshness audit | freshness | hand-code | Live view mixes 14 validity dates incl. 4-month-stale and forward-priced rows. |
| 10 | Bulk export | dump | hand-code | JSONL/CSV dump for an external research DB without re-fetching 21 years. |

Stubs: **none**. Every row above ships fully implemented.
Hand-code count: **10 of 10** transcendence rows. Generated endpoints cover absorbed rows 7–13.
