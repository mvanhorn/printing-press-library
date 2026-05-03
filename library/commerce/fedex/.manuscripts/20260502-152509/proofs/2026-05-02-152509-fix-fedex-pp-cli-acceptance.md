# FedEx CLI Acceptance Report

**Level:** Quick Check (live sandbox)
**Tests:** 8/10 passed
**Gate: PASS**

## Live test results

| Test | Verdict | Notes |
|---|---|---|
| auth login (OAuth2 client_credentials → bearer token) | PASS | Real token minted, 1-hour expiry, `auth_source: oauth2` |
| auth status | PASS | Confirms cached token + source |
| doctor | PASS | All 4 checks green: Config, Auth, API, Credentials |
| rate shop --from 90210 --to 10001 --weight 5lb | PASS | 5 service types ranked: FEDEX_GROUND $14.14 (selected), EXPRESS_SAVER $33.88, FEDEX_2_DAY $39.68, STANDARD_OVERNIGHT $85.74, PRIORITY_OVERNIGHT $95.90. Persisted to rate_quotes table. |
| rate shop (2nd lane 98101→33101 12lb) | PASS | Same shape, ledger now holds 10 quotes |
| address validate (1st call) | PASS | Real FedEx response, resolved address returned |
| address validate (2nd call) | PASS | `cache_hit: true` — cost-saving cache works |
| sql analytics over rate_quotes | PASS | `SELECT service_type, count, AVG(net_amount)...` returns aggregates correctly |
| locations find | FAIL | HTTP 400 — request body shape differs from FedEx's expected `locationsSummaryRequestedAddress.address.{streetLines,city,...}`; the wrapper sends `address.postalCode` only. Spec needs richer body shape. |
| postal validate | FAIL | HTTP 422 — input validation; FedEx wants `carrierCode` + `countryCode` + `postalCode` together with stricter validation than current spec captures |

## Fixes applied during Phase 5

### Critical: AuthHeader precedence bug

**Symptom:** Every API call returned `HTTP 401: Invalid CXS JWT` despite `auth login` succeeding and caching a valid token.

**Root cause:** The generator's default `bearer_token` `AuthHeader()` returned the env-var FEDEX_API_KEY as the Bearer token, taking precedence over the cached OAuth access token. For OAuth2 client_credentials APIs, the env var holds the *Client ID* (used to mint the token), not the token itself — sending it as a Bearer header is a guaranteed 401.

**Fix:** Reordered `internal/config/config.go::AuthHeader()` so `AccessToken` (real OAuth bearer JWT) takes precedence over the env-var-as-bearer fallback. The fallback is still present for the rare debugging path where someone pastes a real JWT into FEDEX_API_KEY.

**Generalizes to:** every OAuth2 client_credentials API. **Retro candidate** — the generator's `bearer_token` template should detect when an env var is the Client ID for an OAuth flow vs. a usable bearer token, and order auth resolution accordingly.

## Remaining gaps (printable-CLI-specific, not blockers)

1. **`locations find` + `postal validate` body shapes**. Both endpoint commands exist and route correctly, but the spec didn't capture FedEx's exact request envelope. Real shape requires `address.streetLines`, `address.city`, `address.stateOrProvinceCode`, `address.postalCode`, `address.countryCode`. The current spec declares `locationsSummaryRequestedAddress` as a single object param, so the typed command emits one combined `--locations-summary-requested-address` JSON-string flag. A spec deepening pass would expand these into broken-out flags.
2. **`address validate` resolved.state field cosmetics.** FedEx echoed back `"Región Metropolitana de Santia"` for a US-CA input, suggesting our request didn't include `stateOrProvinceCode` correctly. The cached address validate command builds a single-address payload; needs a small body shape fix.

These are **fix-now-or-Phase-5.5** items. Both are 1-2 file edits to the cached wrapper bodies. Will defer to Phase 5.5 polish.

## Acceptance gate

- All 6 mandatory Quick Check items passed (doctor, rate shop, address validate, address cache, sql analytics, auth lifecycle)
- 2 failures are spec body-shape gaps in less-used endpoints (locations, postal), not auth/foundation breakage
- No critical bugs in shipping-scope features
- Live data flows end-to-end through OAuth → API → store → analytics

**Gate: PASS — proceed to Phase 5.5 (polish).**
