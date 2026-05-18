# Browser-Sniff Discovery Report — miami-dade-clerk

**Run:** 20260517-201552
**Target:** `https://onlineservices.miamidadeclerk.gov/officialrecords`
**Capture backend:** agent-browser (anonymous, no auth)
**Combined with:** prior empirical recon performed earlier this same session via Playwright MCP (4 successful search executions: 2 Property/Condo, 2 Name/Document)

## Primary user-flow goal

**"Find every recorded document for a Miami-Dade property."** This is what a foreclosure investor or title researcher actually wants — not a single record by ID, but the full set of deeds, mortgages, liens, and lis pendens against a folio.

## Capture summary

### Backend behavior

- Frontend: React SPA on Microsoft IIS — `/officialrecords/` serves the SPA shell, then loads ~30 JS bundles from `/officialrecords/assets/`.
- API base: `https://onlineservices.miamidadeclerk.gov/officialrecords/api/`
- Session: a session cookie `NSC_JOeqtbnye4rqvqae52yysbdjdcwntcw=...` (NetScaler load-balancer cookie) is set automatically on first page load. Persists for the browser session.
- Anti-bot: **reCAPTCHA Enterprise v3** (site key `6LfI8ikaAAAAAH0qlQMApskMGd1U6EqDyniH5t0x`). Tokens are obtained from `https://www.google.com/recaptcha/api2/...` and sent as `x-recaptcha-token` header on search submissions. Token is single-use per search.

### Discovered endpoints

| Path | Method | Purpose | Auth | Captured during |
|---|---|---|---|---|
| `/api/Environment/getStatus` | GET | System status + maintenance window | None | Page load (agent-browser) |
| `/api/home/isLoggedIn` | GET | Auth check (returns `{loggedIn: false}` for public) | None | Page load (agent-browser) |
| `/api/home/isLoggedInIntra` | GET | Same for intranet (public-side, returns false) | None | Page load (prior Playwright MCP) |
| `/api/home/GetDate` | GET | Current server date | None | Page load (agent-browser) |
| `/api/home/documentTypes` | GET | The 80+ doc-type enum (label + 3-letter code) | None | Page load (agent-browser) |
| `/api/settings/basketcounter` | GET | Cart/basket counter (always 0 for anonymous) | None | Page load (agent-browser) |
| `/api/settings/loggedin` | GET | Login state for settings panel | None | Page load (agent-browser) |
| `/api/home/standardsearch` | POST | Submit Name/Document search, returns `{isValidSearch, qs}` | None + recaptcha | Prior Playwright recon |
| `/api/home/propertysearch` | POST | Submit Property/Condo search, returns `{isValidSearch, qs}` | None + recaptcha | Prior Playwright recon |
| `/api/SearchResults/getStandardRecords?qs=<token>` | GET | Fetch result set (500-row cap) | None | Prior Playwright recon |

### Request shape — propertysearch (POST)

All params in querystring (despite being POST):
```
addressNoUnit=5600%20W%2013%20AVE
addressUnit=
dateRangeFrom=
dateRangeTo=
documentType=
searchType=Property/Condo
```
Headers: `x-recaptcha-token` (single-use), `content-type: application/json; charset=utf-8`, standard Chrome UA + sec-ch-* headers.

### Request shape — standardsearch (POST)

```
partyName=GARCIA%20ROGER
dateRangeFrom=
dateRangeTo=
documentType=
searchT=
firstQuery=y
searchtype=Name/Document
```

### Response shape — `{isValidSearch, qs}` envelope

```json
{
  "isValidSearch": true,
  "qs": "DHKMBIY7/UX9SzkUKq2YekiemTdyQSO3i27vLBRy9Setkv/5DEBKgSlpUpStUAKlwt9buOuxSnk1+DVwtNbqnjSCYqgMJTdHIZMGeR0/RVxX9R25auQ424QzE0Kn1HmeuXnKG0anFRraAstAN9D98w=="
}
```

`qs` is a base64-encoded encrypted search token. The SPA navigates to `/officialrecords/SearchResults?qs=<token>` which then calls `getStandardRecords` with the same token.

### Response shape — `getStandardRecords`

```json
{
  "searchCritiriea": { "partyName": "GARCIA ROGER", "documentType": "", ... },
  "recordingModels": [
    {
      "qs": "<per-record encrypted token>",
      "cfN_MASTER_ID": 23892360,
      "cfN_YEAR": 1967,
      "clerk_File": "1967 R 192467",
      "cfN_SEQ": 192467,
      "doC_TYPE": "DEED - DEE",
      "reC_DATE": "12/12/1967 12:00:00 AM",
      "doC_DATE": null,
      "reC_BOOK": 5747,
      "reC_PAGE": 492,
      "firsT_PARTY": "GARCIA ROGER C",
      "seconD_PARTY": "VENTURA ALBERT",
      "foliO_NUMBER": 0,
      "address": null,
      "legaL_DESCRIPTION": null,
      "subdiV_NAME": null,
      "plaT_BOOK": 0,
      "plaT_PAGE": 0,
      "blocK_NO": null,
      "consideratioN_1": 0,
      "deeD_DOC_TAX": 0,
      "documentarY_STAMPS": 0,
      "intangible": 0,
      "doC_PAGES": 1,
      "key": 95248830,
      "keys": "66338003"
      // ... 53 fields total
    }
  ]
}
```

500-row hard cap per response. No native pagination cursor.

## Doc types (from `/api/home/documentTypes`)

Empirically observed 30+ codes in returned records. Full enum from the dropdown (80+ codes):
DEE QCD ODE DAM DM MOR AMO SMO MRE PRM MOR_I MOR_X CMO ASG AIT LIS CLP JUD SJU AJ LNJUD LIE FTL NTL CTI CVP CCP DCP FCP TCP DVP DVJ AFF AGR AFD APB AIN BAN BSA CER CON COV COC DCE DCO DOR DRC DSC DSR DIS DOM EAS FCN FST LEA MAP MIS NTY NOT NCO NCT OPT ORD PRE PLT PAY PAD PRO PCT REL RRS RSL RES TST TAG WAI

## Search-mode behavior (confirmed dual-index model)

| Mode | Returns | Empirical |
|---|---|---|
| **Property/Condo** | DEEDS only (DEE, QCD, DAM, DM, ODE) | "5600 W 13 AVE" → 14 records, ALL deeds |
| **Name + DocType** | Everything else (mortgages, sats, lis pendens, judgments, liens, ASGs, court papers) | "HERRIOTT NATHANIEL" → 55 records spanning 14 doc types |

This confirms the Florida-recording dual-index assumption: property index for deeds, name index for everything name-attached.

## Replayability verdict — **PASS for `browser_clearance_http` mode**

- Tokens are short-lived but obtainable headlessly via the official reCAPTCHA v3 `grecaptcha.execute()` call (no challenge for `score >= threshold`).
- The cookie `NSC_JO*` (NetScaler) is the only persistent state; it's set automatically on first GET.
- Per-request flow:
  1. Fetch page → get fresh reCAPTCHA token via `grecaptcha.execute(siteKey, {action: 'submit'})`
  2. POST search → get `qs` token
  3. GET `getStandardRecords?qs=<token>` → records

The Go CLI must either: (a) run a headless Chromium to obtain reCAPTCHA tokens, OR (b) ask the user to `auth login --chrome` once to capture the session and proceed with persistent cookies + fresh tokens per query. Recommend option (b) for the printed CLI — simpler, more reliable, lower runtime cost.

## Proxy pattern detection

**NO.** Each endpoint has its own path under `/officialrecords/api/`. Not a proxy-envelope shape. Standard REST surface with mixed path conventions (some PascalCase like `Environment/getStatus`, some camelCase like `home/standardsearch`, some lowercase like `settings/basketcounter`).

## Findings to feed into spec authoring (Phase 2)

1. Base URL: `https://onlineservices.miamidadeclerk.gov/officialrecords`
2. Three primary endpoints for the CLI: `standardsearch`, `propertysearch`, `getStandardRecords`
3. Two utility endpoints: `Environment/getStatus`, `home/documentTypes`
4. Auth type: `cookie` (NetScaler cookie) + `header` (reCAPTCHA token per request) = `composed` per SKILL spec extensions
5. Per-record viewer URL: `/officialrecords/Document?qs=<per-record-qs>`
6. Money is in DOLLARS (convert to CENTS for our output)
7. Dates are in `M/D/YYYY 12:00:00 AM` format (no zero-pad) — parser must handle this
8. Folio is 12-digit numeric in clerk responses (drops leading zero) — pad to 13 for our APN matching

## Time budget used

- agent-browser capture: 30 seconds
- Synthesis with prior recon: 60 seconds
- Total: ~90 seconds (well under 3-minute SKILL budget)
