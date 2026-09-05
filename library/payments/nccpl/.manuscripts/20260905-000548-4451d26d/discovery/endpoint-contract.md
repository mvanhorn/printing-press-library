# NCCPL endpoint contract — derived from page JS, 5 Sep 2026

**Source of truth:** the inline JavaScript of `GET /market-information`, extracted from the
sanitized HAR (2,002,406 bytes, base64-decoded). This is NCCPL's own request-building code,
not inference. No network access was used to produce this document.

## Auth
- All 21 `/data` endpoints: `POST`, `Content-Type: application/json`, header
  `X-CSRF-TOKEN: <token>` where the token comes from `<meta name="csrf-token">` on
  `/market-information`. Confirmed present in the captured page head.
- Laravel also accepts `X-XSRF-TOKEN` = URL-decoded `XSRF-TOKEN` cookie (independently
  confirmed by `hmehmood56-debug/PSX-Trader`). Prefer the cookie route — no HTML parse.
- All `GET /api/<r>/latest-date` endpoints: no CSRF, clearance only.
- Cloudflare `cf_clearance` cookie required for every request.

## Date conventions — THREE different formats in one API
| Endpoint group | Field(s) | Format | Evidence |
|---|---|---|---|
| 16 single-date `/data` | `date` | `YYYY-MM-DD` | `date: selectedDate`, `selectedDate = dateFilter.value` (native `<input type=date>`), no conversion |
| `fipi/data`, `lipi/data` | `fromDate`,`toDate` | **`DD/MM/YYYY`** | `apiFromDate = toApiDateFormat(fromDate)` at lines 4455-4456, 4948-4949 |
| `fipi-sector-wise/data`, `lipi-sector-wise/data` | `fromDate`,`toDate` | `YYYY-MM-DD` | `fromDate: fromDate` raw; guard uses lexicographic `fromDate > toDate`, valid only for ISO |
| `graph-data/data-by-date-range` | `start_date`,`end_date` | `YYYY-MM-DD` | multipart/form-data, observed in HAR |

`toApiDateFormat` source comment: `// Native date input gives YYYY-MM-DD; API expects DD/MM/YYYY.`

Encoding the wrong format per group is a silent-empty-result bug. Do not unify these.

## Response envelope key per endpoint — NOT uniform
All responses are `{"success": <bool>, "<key>": [...]}`.

| Endpoint | Body keys | Envelope key |
|---|---|---|
| `/api/fipi/data` | fromDate,toDate,type | `data` |
| `/api/lipi/data` | fromDate,toDate,type | `data` |
| `/api/fipi-sector-wise/data` | fromDate,toDate | `data` |
| `/api/lipi-sector-wise/data` | fromDate,toDate | `data` |
| `/api/fipi-normal/data` | date | **`records`** |
| `/api/lipi-normal/data` | date | **`data`** |
| `/api/open-positions/data` | date | `positions` |
| `/api/mfs-open-position/data` | date | `positions` |
| `/api/msf-open-position/data` | date | `positions` |
| `/api/financiers-financees/data` | date | `records` |
| `/api/force-release/data` | date | `records` |
| `/api/top-15-financiers/data` | date | `records` |
| `/api/mts-amount-refinanced/data` | date | `records` |
| `/api/mfs-top-15-financees-and-financiers/data` | date | `records` |
| `/api/msf-top-15-financees-and-financiers/data` | date | `records` |
| `/api/slb-market-information/data` | date | `rows` |
| `/api/un-listed-tfc/data` | date | `tfc` |
| `/api/sett-info-uin-wise/data` | date | `sett` |
| `/api/sett-info-cm-wise/data` | date | `sett` |
| `/api/var-margins/data` | date | `margins` |

**`fipi-normal` returns `records` while `lipi-normal` returns `data`.** This asymmetry is in
NCCPL's own code (line 883 vs 2133). A client that assumes symmetry silently returns empty
for one of them.

## Investor type codes (`type` param, fipi/lipi only)
FIPI (`typeFilter`): `101` ALL · `FI` Foreign Individual · `FC` Foreign Corporates ·
`FOP` Overseas Pakistani · `FN` FIPI Net

LIPI (`lipiTypeFilter`): `101` ALL · `LBD` Banks/DFI · `LBP` Broker Proprietary Trading ·
`LC` Companies · `LI` Individuals · `LN` LIPI Net · `LMF` Mutual Funds · `LNB` NBFC ·
`IC` Insurance Companies · `LOO` Other Organization

These map 1:1 onto Portfolio360's matrix columns (Foreign / Individuals / Mut. Funds /
Banks-DFI / Companies / Brokers / Insurance / Other).

## Other facts
- `fipi`/`lipi` rows carry `segment` ∈ {`EQUITY`, `DEBT`}; the page splits and subtotals on it.
- `latest-date` responses: `{"latest_date":"YYYY-MM-DD","success":true}`.
- Per-resource freshness is NOT in lockstep. Observed 2026-09-04: fipi/lipi/most = `2026-09-04`,
  `slb-market-information` = `2026-08-27`, `msf-*` = `2026-08-21`, `un-listed-tfc` = `2026-05-22`.
- `graph-data/latest-data?graph_type=value|volume&limit=N` →
  `{"data":{"graph_type","total_records","value_data":[{"x":date,"y":number}]}}`.

## Unresolved
- Maximum accepted range width on the `{fromDate,toDate}` endpoints. `PSX-Trader` probes a
  one-year window, implying wide ranges work, but this is untested here. Backfill must request
  wide and narrow on failure rather than assume.
