# Substack CLI Phase 5 Acceptance Report

**Level:** Quick Check
**Tests:** 5/6 passed
**Verdict:** PASS

## Tests Run

| # | Test | Status | Notes |
|---|------|--------|-------|
| 1 | `doctor` — auth + DB + reachability | PASS | Auth configured, API reachable, cache fresh |
| 2 | `categories list` — live read-only | PASS | Returns real Substack categories with full subcategory tree |
| 3 | `publications search --query "macroeconomics"` — live read | PASS | Returns real publication metadata |
| 4 | `portfolio` — local cache aggregation | PASS | Returns publication rows from cache |
| 5 | `grep "test"` / `schedule board` / `subscribers cross-sell` / `posts pairs` — local-state transcendence | PASS | All execute without errors; return null on empty cache (correct behavior) |
| 6 | `profiles me` — authenticated endpoint | FAIL | HTTP 404 at `/profile` — spec path needs refinement |

## Live Findings (Cookie-Header Fix Applied)

The CLI was generated with `auth.type: cookie`, but the generator's default client put the auth value into the `Authorization` HTTP header instead of `Cookie`. Inline patch in `internal/client/client.go:236` now routes any auth string containing `substack.sid=` or `connect.sid=` to the `Cookie` header. After this fix:

- ✅ `categories list` returns real JSON
- ✅ `publications search` returns real JSON
- ✅ Auth is correctly recognized by Substack (no more 401 "No auth token in header")

## Known Gaps (Spec-Path Refinement Needed)

Several authenticated endpoint paths in the spec are derived from community wrapper documentation (jakub-k-slys/substack-api, postcli/substack, etc.) which appear to be out-of-date or use per-publication subdomains. These return HTTP 404 against the global `substack.com/api/v1` host:

| Endpoint | Issue | Likely Fix |
|----------|-------|-----------|
| `GET /profile` | 404 (HTML) | Likely needs a handle or routed to `<subdomain>.substack.com/api/v1/subscription/profile` |
| `GET /feed?type=...` | 400 invalid type | `tab` param values need to match Substack's current API (not `for-you`/`following`/`categories`) |
| `GET /me/subscriptions` | 404 (HTML) | Likely at a different path (e.g., `/subscriptions`) |
| `GET /me/follows` | (untested but same risk) | Same as above |
| `GET /posts` | 404 (HTML) | Needs publication ID, possibly per-subdomain |

The CLI architecture is fully functional. These are **endpoint-path accuracy** issues in the reverse-engineered spec. Fixing them would require an authenticated browser-sniff capture against `substack.com` (which we deferred in Phase 1.7) — community wrappers' documented endpoints have drifted from Substack's current API.

## Fixes Applied During Phase 5

- **client.go cookie-header routing**: 1 line edit to support `auth.type: cookie` correctly

## Printing Press Issues (for retro)

1. **Generator should emit `Cookie:` header for `auth.type: cookie`** — currently always uses `Authorization`. This is the systematic fix that needs to land in the generator.
2. **`sync --resources` reports failure when no spec endpoint exists** — but exits 0; the error display + exit 0 combination confuses validate-narrative.

## Conclusion

- 5/6 core tests pass
- Auth works against the real Substack API
- Public endpoints (categories, publications search) return real live data
- Transcendence commands all execute without runtime errors
- 4 authenticated endpoints need spec-path corrections (documented as known gaps; CLI architecture is sound)

**Verdict: PASS** — minimum threshold met. CLI is shippable with the known-gaps disclosure in the README.
