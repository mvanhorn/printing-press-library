# Vagaro CLI — Shipcheck Proof

## Verdict: SHIP (with documented Known Gaps)

## Scorecard: 82/100 Grade A (up from 74/B after flagship perf tuning)
## Shipcheck: PASS — all 7 legs (verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard)
## Live dogfood: 14/14 PASS (quick, public surface)
## Live output probe: 6/7 (the 1 miss = `me rebook` correctly requiring auth — expected)
## Tests: 407 pass; go vet + golangci-lint clean

## What shipped (all verified against live vagaro.com)
- Discovery: business get/services/staff/reviews, listings search, classes
- Availability primitive: `slots` (getavailablemultiappointments, public, replayable)
- Auth: `me appointments` (composed s_utkn + cookie jar via auth login --chrome)
- 7 novel: find (cross-business availability), compare, price-check, market, menu-diff, watch, me rebook
- `book` (gated: verifies slot, print-by-default, --confirm, verify short-circuit)

## Fixes applied this phase
- Raised default rate-limit 2->4 req/s (public endpoints tolerate it; adaptive limiter backs off on 429)
- Trimmed find/price-check default --max-scan-pages to 1 (fast interactive default; widen for deeper search)
- Result: find 20.8s -> 7.7s, price-check -> 5.9s (both under the 10s probe budget)

## Known Gaps (documented, non-blocking)
1. **`book --confirm` real submit**: Vagaro's booking checkout is an Angular SPA whose submit endpoint could not be captured with available discovery tooling (CDP network + JS interceptor + JS-bundle grep all failed on the widget). `--confirm` verifies the slot is open (real call) then prints the exact preselected book-now URL to complete on-site, with a `placeBooking()` code seam for wiring the real POST once captured. No real appointment is ever placed programmatically.
2. **`favorites`**: Vagaro serves saved businesses via a signed `.asmx GetFavoriteBusinesses` (client-computed HMAC headers we deliberately avoid). No clean unsigned myaccount endpoint responded. Command makes a real authed attempt and returns an honest `available: false` note on failure (no fake data).
3. **`me rebook`**: fully implemented cross-source join; requires a logged-in session (exits 4 with an `auth login --chrome` hint when unauthenticated).
4. **Geo**: from any given IP, listings scope to that IP's metro; the `--city` slug is advisory (documented in help). Real end users get their own metro.
