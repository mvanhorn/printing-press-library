# Acceptance Report: NSE India CLI

**Level:** Full Dogfood  
**Tests:** 12/12 passed  
**Gate:** PASS

## Test Matrix

| # | Test | Result | Notes |
|---|------|--------|-------|
| 1 | doctor (health check) | PASS | API reachable, auth not required |
| 2 | market --json | PASS | Capital Market: Open, NIFTY at 24,194 |
| 3 | equity quote ADANIPORTS --json | PASS | LTP: 1,756.20, 5-level order book |
| 4 | equity derivatives ADANIPORTS --json | PASS | 3 futures + 79 options contracts |
| 5 | corporate actions ADANIPORTS --json | PASS | ₹7.50 dividend, ex-date 12-Jun-2026 |
| 6 | corporate announcements RELIANCE --json | PASS | 3,280 filings returned |
| 7 | corporate financial-results TCS --json | PASS | 162 quarterly filings with XBRL links |
| 8 | corporate insider-trading ADANIPORTS --json | PASS | 20 PIT disclosures |
| 9 | indices list --json | PASS | 146 NSE indices |
| 10 | indices constituents "NIFTY BANK" --json | PASS | 15 bank stocks with live prices |
| 11 | symbol-lookup TATA --json | PASS | 18 TATA symbols found |
| 12 | corporate annual-reports INFY --json | PASS | 15 reports with direct PDF links |

## Fixes Applied

1. **HTTP/2 transport** — Generated client used Go's default HTTP/2 transport; NSE India returns INTERNAL_ERROR on H2 streams. Fixed by forcing HTTP/1.1 transport in `internal/client/client.go`.
2. **User-Agent header** — Generated client sent `github.com/mvanhorn/nse-india-pp-cli/0.1.0`; NSE blocks non-browser User-Agents. Fixed to `Mozilla/5.0 ... Chrome/120.0.0.0`.
3. **Referer header** — Added `Referer: https://www.nseindia.com/` (required by NSE backend).
4. **Doctor probe endpoint** — Doctor was probing `https://www.nseindia.com/` (HTML homepage, also H2); fixed to probe `/api/marketStatus`.
5. **Module naming** — Generator produced `module NSE India-pp-cli` with spaces; fixed to `github.com/mvanhorn/nse-india-pp-cli`.
6. **`search` resource name collision** — Renamed `search` resource to `symbol_lookup` to avoid collision with Printing Press's reserved `search` template.

## Printing Press Issues (for retro)

- The spec `name` field with spaces (`NSE India`) propagates to `module NSE India-pp-cli` in go.mod — the generator should sanitize the module name from the slug, not the display name.
- The doctor health check probes `"/"` — this should be configurable or default to a lighter API endpoint when the base URL is a website.
- Generated HTTP client doesn't emit `transport.headers` from the spec even when they're specified — should apply them automatically.
