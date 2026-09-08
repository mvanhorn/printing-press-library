# MUFAP CLI Brief

All claims below are measured, not assumed. Evidence commands in `../discovery/`.

## API Identity
- Domain: Mutual Funds Association of Pakistan — industry body, not an exchange.
- Surface: ASP.NET MVC, server-rendered HTML tables. No JSON API on the data paths.
- Users: AMCs, distributors, retail investors, SECP.
- Reachability: `probe-reachability` -> **`browser_http`** (confidence 0.85).
  stdlib HTTP = 403 `cf-mitigated: challenge`; **Surf/Chrome-TLS = 200**.
  `needs_clearance_cookie: false`, `needs_browser_capture: false`.
  NOT the NCCPL case. No cf_clearance, no headed Chrome, no CDP. Ship Surf transport.

## The query contract (ground truth read from the page's own JS, not inferred)
`window.location.assign` in the inline block of `/Industry/IndustryStatDaily`:

    /Industry/IndustryStatDaily?tab={tab}&AMCId={amc}&fundId={fund}&datefrom={YYYY-MM-DD}&datetill={YYYY-MM-DD}
    /Industry/IndustryStatMonthly?tab={tab}&AMCId=..&fundId=..&datefrom=..&datetill=..

- Date format is `YYYY-MM-DD` via `toIndustryIsoDate()`. Matches psx-research storage convention.
- Plain **GET**, server-rendered HTML. The encrypted `ApiPayloadCrypto.js` path is used ONLY
  for the AMC/Fund dropdowns and `getAssetAllocationById`. The panel data needs no crypto.
- TRAP FOUND AND CORRECTED IN-RUN: `filterDate` (the visible `<input type=month>` id on the
  monthly page) is NOT the query param. Using it returns HTTP 200 with a byte-identical
  snapshot for every month — a silent-same bug. The real param is `datefrom`/`datetill`.
  My first monthly test used `filterDate` and produced a FALSE NEGATIVE ("not backfillable").
  Re-tested with the JS-derived params: it backfills correctly.

## Tabs (all share the same date contract)
| tab | content |
|-----|---------|
| 1,2 | NAV + return series: YTD, MTD, 1/15/30/90/180/270/365 day, 2Y, 3Y |
| 3   | Offer / Repurchase / NAV, Front-end, Back-end, Contingent load, Trustee |
| 4   | **Payout (per unit), Ex-NAV, Payout Date** — distributions |
| 5   | **TER MTD%, TER YTD%, MF%, S&M%** — expense ratios |

## Backfill — MEASURED, this is the headline result
Daily panel, `tab=1`, one request per date, universe width printed each time:

    2005-06-10 ->  17 funds     2019-08-14 -> 227
    2008-06-10 ->  66           2022-03-15 -> 285
    2010-06-10 -> 106           2025-06-16 -> 393
    2014-06-10 -> 163           2026-09-04 -> 388

**21 years of daily history, fully backfillable.** Distinct NAV hashes per date; the
`Validity Date` column always equals the requested date.

Monthly AUM (`IndustryStatMonthly`, net assets PKR mn per fund):

    2018-06-30 -> 222 funds, PKR   524,093 mn
    2022-06-30 -> 285 funds, PKR 1,191,920 mn
    2025-12-31 -> 438 funds, PKR 4,005,405 mn
    2026-07-31 -> 526 funds, PKR 4,478,763 mn

Distinct hashes, monotone-plausible growth. Real series.

## Data quality — the raggedness finding
An **explicit** date query returns a CLEAN single-validity-date panel:
  Thu 2026-09-03 -> 524 rows, 1 distinct validity date
  Fri 2026-09-04 -> 388 rows, 1
  Sat 2026-09-05 ->  42 rows, 1   (weekend: only some funds price)
  Wed 2025-06-18 -> 393 rows, 1

Only the **current-day default view** is ragged: Sun 2026-09-06 returned 551 rows across
**14 distinct validity dates** (Sep 04:345, Sep 03:135, Sep 07:22 forward-dated, ... May 18:2
i.e. ~4 months stale). That view is "latest available per fund", not a daily observation.
=> Backfill is clean. The live/current path must key on `Validity Date`, never on fetch date.
Naive daily differencing of the current view would manufacture fake returns.

## Universe composition (551 funds, 2026-09-06)
    Open-End Funds              396
    Voluntary Pension (VPS)      88
    Employer Pension Funds       53
    Exchange Traded Fund (ETF)    8   <- the ONLY rows with a PSX ticker
    Dedicated Equity Funds        6

## Validation: money-market yields vs the known SBP policy-rate cycle
Median 30-day annualized return of Money Market category, by date:

    2020-08-14   5.03   (SBP 7.00, COVID trough)
    2021-08-13   7.18   (7.00)
    2022-08-12  14.44   (15.00)
    2023-08-11  20.85   (22.00, peak)
    2024-08-09  20.25   (19.50, cuts begin)
    2025-08-15   9.64   (11.00)
    2026-09-04  10.68   (~11)

Shape matches the policy cycle exactly; level sits slightly below policy, which is the
expected sign (net of TER, T-bill-backed). This is a *proxy*, not the policy rate.

## Value to psx-research — see the scope note; summarised
1. Daily market-implied PKR short rate from MM-fund yields. ~2,500 obs since 2016,
   ~5,000 since 2005. Fills `rates.py`, which is EMPTY because SBP is Cloudflare-blocked (0ak).
2. Daily aggregate equity-fund return -> active-vs-index spread. ~2,500 obs.
3. Monthly AUM + monthly net sales = the NCCPL MUTUAL-FUNDS-class join. **Monthly, ~120 obs.**
4. Only 8 ETFs join `daily_bars` on `(symbol, date)`.

## Corrections to the assumptions in the run request
- "Fund holdings/NAVs key (symbol,date) the same way as daily_bars" — **false for 543 of 551
  funds.** Open-end funds have no PSX ticker. Only the 8 ETFs join by symbol.
- "MUFAP AUM/NAV is the other side of the NCCPL MUTUAL FUNDS flow trade" — **true, but AUM is
  MONTHLY.** ~120 observations since 2016: the same underpowered regime that killed SUE
  (11 cohorts) and the nowcast (22 names). Daily NAV is not AUM.

## Data layer
- Primary key: `(resource, date, row_key)` where `row_key` = fund name, `date` = **Validity Date**.
- Coverage ledger recording a date even when it returns ZERO rows (Lesson 4).
- `observed_at` stamped at fetch (publication timing is what makes a variable ex-ante).
- Universe width persisted per date (Lesson 8) — it moves 17 -> 526 and is itself a regime marker.

## Arithmetic invariants available for a `verify` command
- Industry AUM total == sum of category AUM == sum of member-fund AUM (monthly).
- `Offer >= NAV >= Repurchase` (tab 3) modulo load structure.
- Ex-NAV + Payout == cum-NAV on the payout date (tab 4).
- NOT available: units outstanding, so "NAV x units = net assets" cannot be checked.
