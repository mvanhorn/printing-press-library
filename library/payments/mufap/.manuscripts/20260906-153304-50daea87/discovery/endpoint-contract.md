# MUFAP endpoint contract — derived from the site's own JS, then measured

Transport: **Surf / Chrome-TLS**. stdlib HTTP = 403 Cloudflare challenge; Chrome TLS = 200.
No cf_clearance, no browser, no CDP. `probe-reachability` -> `browser_http`.

## A. HTML surfaces (GET, server-rendered tables)
Contract read from inline `window.location.assign` call sites:

    /Industry/IndustryStatDaily?tab={1..5}&AMCId=0&fundId=0&datefrom={YYYY-MM-DD}&datetill={YYYY-MM-DD}
    /Industry/IndustryStatMonthly?tab={1..}&AMCId=0&fundId=0&datefrom={YYYY-MM-DD}&datetill={YYYY-MM-DD}
    /Industry/WebMonthlyNetSales
    /Industry/WebUnitHolderPattern
    /FundProfile/FundDirectory

tab1/2 NAV+returns · tab3 offer/repurchase/loads · tab4 payout/ex-NAV · tab5 TER

## B. JSON surfaces (POST, **plain JSON — encryption is OFF**)
`API_PAYLOAD_ENCRYPTION_ENABLED = 'false'` on public pages, so `ApiPayloadCrypto` takes the
documented "plain JSON fallback (public-safe)" branch. Payloads are NOT encrypted.
The *URL map* is AES-256-CBC (`enc:` prefix, fixed IV `0000000000000000`, key from
`window.__mufapCfg.k` — a placeholder value, not a credential). Decrypted map, 26 endpoints:

    /AMC/GetAMCList                     {}                            -> 27 AMCs, AMCId = GUID
    /TopHolding/GetFundNameByAMC        {AMCId: <guid>}               -> funds: FundID(guid) + fund(int)
                                                                         + Cat_Desc + PricingMechanism
    /Industry/GetAssetAllDetailbyId     {ID: <fund int>, Date:"M-YYYY"} -> asset allocation
    /Industry/GetUnitPatternHolder      {Date: "YYYY"}                -> investor/sector unit pattern
    /Industry/GetDateList               {}                            -> available Year/Month
    /WebPost/GetAnnouncementPayoutData  -> payout announcements
    /VoluntarilyPensionSch/WebBreakUpAgeJson | WebBreakUpCashJson | WebWithDrawalCashJson
    /AMC/GetFundDetailbyAMC | GetFundDetailbyAMCByDate   (500 on the params tried; unused)

## C. FOUR distinct date encodings (cf. NCCPL's three)
    1. YYYY-MM-DD     HTML query params (datefrom/datetill)
    2. "Mon DD, YYYY" Validity Date, as displayed in the table
    3. M-YYYY         GetAssetAllDetailbyId  (e.g. "7-2026" — NOT zero padded, NOT ISO)
    4. YYYY           GetUnitPatternHolder
Wrong encoding = HTTP 200 with an empty table, or HTTP 500. Never an informative error.

## D. TRAPS — every one of these was hit and measured during discovery
1. **`filterDate` is not a query param.** It is the visible `<input type="month">` id on the
   monthly page, but the real params are `datefrom`/`datetill`. Passing `filterDate` returns
   HTTP 200 with a byte-identical snapshot for EVERY month. Produced a false "not
   backfillable" verdict on first pass; corrected by reading the JS.
2. **`message: "No data found"` appears even when `data` is fully populated.** Never gate on it.
   Gate on the parsed row count.
3. **`data` is a JSON *string*** for GetAssetAllDetailbyId — needs a second `JSON.parse`.
4. **`ID` is the integer `fund` code, not the FundID GUID.** GUID -> HTTP 500. Non-GUID
   junk ("1") -> HTTP 200 with an empty Table. Two different failure shapes for the same mistake.
5. **`*Percent` fields are 0.0 for everything before ~2024, while the AMOUNT fields are correct.**
       6-2020  StocksOREquities=4345.55  StocksOREquitiesPercent=0.0
       6-2016  StocksOREquities=3021.00  StocksOREquitiesPercent=0.0
       6-2012  StocksOREquities= 238.03  StocksOREquitiesPercent=0.0
   Any historical study keyed on the percent columns silently gets all zeros.
   **Use the amount fields and derive the percentage as amount/Total.**
6. **`TotalPercentage` is the literal string `"100%"`, not a computed check.** It reports
   "100%" even when the components do not sum to 100 (see invariant below).
7. **The current-day HTML view is ragged**: 551 rows across 14 validity dates, incl. rows
   ~4 months stale and forward-dated rows. Explicit historical dates return clean single-date
   panels. `PricingMechanism: "forward"` on many funds explains the forward-dated NAVs.

## E. Arithmetic invariants (for the `verify` command)
`Total` is net assets (AUM). Verified: Stocks/Total reproduces the reported percent exactly
(7-2026: 10005/10514 = 95.16%; 6-2024: 3605/3455 = 104.34%).

    INVARIANT: sum(asset-class %) - liabilities% == 100
      7-2026  101.97 - 1.97 = 100.0000  PASS
      6-2026  101.09 - 1.09 = 100.0000  PASS
      3-2026  103.82 - 1.77 = 102.0500  **FAIL by 2.05pp — while TotalPercentage still says "100%"**

A date that fails this is corrupt input, not a weak signal. Ship it as `verify`.

## F. Backfill depth, measured
    Daily NAV panel (HTML)      2005 -> present   (17 funds 2005 -> 388 in 2026)
    Monthly AUM (HTML)          2018 tested OK; earlier untested
    Asset allocation (JSON)     2012 -> present   (6-2008 returns no data)

## G. ROW KEY — measured 6 Sep, after the parser was written
Fund name is NOT unique within a date. On 2026-09-04, tab=returns, 388 rows:

    row_key candidate              unique   collisions
    Fund Name                        339        49
    Sector | Fund Name               339        49
    Sector | Category | Fund Name    387         1     <- USE THIS

Two distinct causes:
1. **VPS pension funds legitimately repeat their name.** "ABL Pension Fund" is THREE rows —
   VPS-Money Market (NAV 293.7521), VPS-Debt (388.4842), VPS-Equity (647.2226). Same fund
   family, three genuinely different sub-fund series. Keying on name collapses them into one
   and silently discards two thirds of the pension universe.
2. **One genuine upstream duplicate.** "Pak Qatar Daily Dividend Plan" appears twice with
   identical sector, category, rating and NAV. That is the single residual collision under
   the composite key; disambiguate with an occurrence index within the (resource, date).

Keying on `Fund Name` alone would drop 49 of 388 rows per date — 12.6% — via the
`PRIMARY KEY (resource, date, row_key)` upsert, with no error raised anywhere.

## H. Per-tab column differences (do not assume uniformity)
    tab       rows  name column   has "Validity Date"
    returns    388  Fund Name     yes
    nav        388  Fund          yes
    pricing    551  Fund          yes
    payout      10  Fund          **NO — the date column is "Payout Date"**
    ter        551  Fund          yes

- The name column is "Fund Name" on tab=returns and "Fund" on all four others. Resolve by
  probing which header is present rather than by tab number.
- tab=payout has NO Validity Date column at all; its date is "Payout Date". Any code that
  normalizes the row's own validity date must special-case this tab or it will store nothing.
- tab=pricing and tab=ter return 551 rows for an explicit date while tab=returns returns 388:
  those two tabs are current reference data and are NOT date-filtered the same way. Do not
  read their row counts as a universe width for that date.

## I. ACCOUNTING NEGATIVES — the most dangerous finding of the run
MUFAP writes every negative number in ACCOUNTING NOTATION and never with a minus sign:

    "YTD": "(4.97)"      means  -4.97
    "30 Days": "(3.10)"  means  -3.10

Measured 2026-09-04, tab=returns, 388 rows:
    cells with parenthesised YTD ... 96  (24.7%)
    cells with a leading minus ......  0

A numeric parser that does not decode the parentheses returns "did not report" for every
LOSING fund, so the fund silently leaves the cross-section. The bias is not random: it removes
exactly the left tail.

Measured impact before the fix, equity YTD dispersion on 2026-09-03:
    fund_count   17  of 91 equity funds   (81% dropped)
    median    +3.15
after decoding parentheses:
    fund_count   91
    median    -3.70

**The reported sign of the equity market was inverted.** Anything built on the pre-fix numbers
would have concluded the market was up 3.15% on a day it was down 3.70%.

Money-market rows are almost never parenthesised (2 of 120 on that date), so the short-rate
proxy was unaffected — which is why the SBP-cycle validation still held. The damage was
confined to the categories that actually fall, which is precisely what makes it hard to see.

Handled in `mufap.ParseNumber`, with regression cases for "(4.97)", "(1,236.28)", "(3.10)%",
"()" and "(N/A)".
