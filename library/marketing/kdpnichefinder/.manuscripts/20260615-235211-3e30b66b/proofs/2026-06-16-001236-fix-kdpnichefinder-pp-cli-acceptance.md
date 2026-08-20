# KDP Niche Finder CLI — Phase 5 Acceptance Report

Level: Full Dogfood (live, authenticated browser session)
Gate: PASS — binary live dogfood 71/71 (phase5-acceptance.json status=pass, level=full)

## Auth
Captured the authenticated viewer's logged-in kdpnichefinder.com session via `auth login --chrome` (auto-detected the correct Chrome profile). doctor: all green (auth configured, API reachable, credentials valid).

## Live behavioral results (against real data)
- refresh: mirrored 40 real books across all 4 buckets + wrote daily snapshot. PASS
- niches <type>: live Inertia HTML data-page parse returned real books (validated ParseDataPage against the live site, not just fixtures). PASS
- rank --sort value/opportunity: returned genuinely ranked niches with real revenue/price. PASS
- dupes: found 4 real cross-bucket ASIN matches. PASS
- keywords: real KDP keyword frequencies (bill, organizer, tracker, seniors...). PASS
- competitors <id>: focus book + competitor set returned. PASS
- export: CSV (default) and JSON (--json) both valid on real data. PASS
- drift: correct honest-empty (only one snapshot date so far; needs a 2nd refresh on a later day). PASS
- folders create (CSRF write): created a real folder (validates X-XSRF-TOKEN header end-to-end). PASS
- folders list / categories list / user get: live JSON. PASS

## Fixes applied during Phase 5
1. **Generator bug (fix-before-ship + retro):** generated `cookie` auth never sent the Cookie header — `client.New` used a nil jar, the cookie block was comment-only, and no LoadCookieJar exists. Fixed `internal/client/client.go` to set the Cookie header from the stored cookie string when no jar is present. This unblocked ALL authenticated functionality. RETRO CANDIDATE.
2. Added `Example:` to all 7 novel commands + refresh (dogfood help-check requires examples).
3. `export` now honors `--json` (JSON array) and defaults to CSV — fixes dogfood json-fidelity.

## Known limitations (honest, not bugs)
- saturation (publisher concentration) and competitors' same-publisher match are weak for all-indie buckets: KDP low-content books are almost all "Independently published", so publisher-HHI saturates at 1.0. The price-band signal in competitors and the revenue ranking remain useful. Documented in README.
- A test folder ("pp-livetest", id 1384) was created during CSRF validation; the CLI has no folder-delete endpoint, so the authenticated viewer can remove it via the web UI if desired.

## Printing Press issues for retro
- Cookie-auth client never sends the Cookie header (nil jar + comment-only block + missing LoadCookieJar). High impact: breaks every cookie-auth CLI's live functionality.
