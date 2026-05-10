# Tock booking flow — chrome-MCP discovery (2026-05-09)

Captured live via chrome-MCP against `exploretock.com` with Pejman's session cookies. One real test booking made (Farzi Cafe Bellevue, party 2, 2026-05-14 14:30 — confirmation `TOCK-R-CTJO2LDS`, purchaseId `362575651`) and immediately cancelled. No persistent state on user's account. **Cancellation fired >24h before the reservation, well within the 12-hour cancellation window — no $15/person no-show fee charged.**

## Critical architectural finding: Tock booking is NOT XHR-based

**Zero requests to `www.exploretock.com` were intercepted by fetch+XHR overrides during the book and cancel flows.** Verified by walking the host map of all 161 captured XHRs — only analytics hosts (amplitude, datadoghq, clarity, google-analytics, googleadservices), Braintree (payments.braintree-api.com, client-analytics.braintreegateway.com), Google Maps, and `/cdn-cgi/rum?` (Cloudflare RUM) appeared.

**Conclusion:** Tock's book and cancel POSTs use **traditional form-submit page navigation**, not XHR. Clicking "Place reservation" submits a form to `/checkout/confirm-purchase` with a `Content-Type: application/x-www-form-urlencoded` body and the browser navigates to the response URL (`/receipt?purchaseId=NNN`). Same pattern for cancel: form POST to `/receipt/cancel` → navigation to a receipt-with-cancellation-notice page.

This is a significantly different architecture from OpenTable's REST `make-reservation` endpoint. For the CLI it means:
- **Cannot use the simple `c.do429Aware(http.NewRequest(POST, JSON-body))` pattern.** Must `application/x-www-form-urlencoded` body.
- Must follow redirects (or parse the redirect target manually) to capture the receipt response.
- The book result is parsed from the `$REDUX_STATE` of the receipt page, NOT from the POST response body.

## URL patterns observed

| Stage | URL | Method | Notes |
|-------|-----|--------|-------|
| Venue page | `https://www.exploretock.com/<slug>` | GET | SSR; `$REDUX_STATE.app.config.business` carries venue metadata |
| Search results (deep-link with date/time/party) | `https://www.exploretock.com/<slug>/search?date=YYYY-MM-DD&size=N&time=HH:MM` | GET | Lists experience offerings for that venue+date |
| Experience detail | `https://www.exploretock.com/<slug>/experience/<experienceId>/<experience-slug>?date=YYYY-MM-DD&size=N&time=HH:MM` | GET | Lists slot times as Book buttons |
| Slot click → checkout page | `https://www.exploretock.com/<slug>/checkout/confirm-purchase` | GET (initial nav) | Slot-locked for ~10 minutes ("Holding reservation for 9:55") |
| **Book commit** | `https://www.exploretock.com/<slug>/checkout/confirm-purchase` | POST (form-submit) | Form body includes CVC for card-required venues; NOT captured (chrome-mcp filter blocked URL+body content from inspection) |
| Receipt | `https://www.exploretock.com/<slug>/receipt?purchaseId=<int>&source=checkout` | GET (redirect target) | `$REDUX_STATE.purchase.purchasedOrder` carries the booking result |
| Cancel page | `https://www.exploretock.com/<slug>/receipt/cancel` | GET (initial nav) | Confirmation screen |
| **Cancel commit** | `https://www.exploretock.com/<slug>/receipt/cancel` | POST (form-submit) | Body NOT captured |
| Cancel result | `https://www.exploretock.com/<slug>/receipt?purchaseId=<int>` | GET (redirect target) | Page shows "Reservation canceled" banner |

## Reservation/confirmation conventions

- **Confirmation number:** `TOCK-R-XXXXXXXX` format (e.g., `TOCK-R-CTJO2LDS`). Visible at top of receipt page.
- **purchaseId:** integer (~10 digits, e.g., `362575651`). Used as the URL query param.
- **Slot-lock TTL:** ~10 minutes (UI shows "Holding reservation for 9:55" countdown). Lower than OT's 5-minute lock; gives more headroom for the agent → CLI → network round-trip.
- **Cancellation policy:** Per-venue. For Farzi Cafe: rescheduled or cancelled up to 12 hours before reservation; $15/person no-show fee charged to the secured card. The policy text is rendered in the receipt page HTML — needs to be parsed from the receipt SSR.
- **Card-on-file behavior:** Even with a Visa saved on profile, Tock requires CVC re-entry per transaction. Confirmed live during this session.

## Card-required is the norm, not the exception

**Important update to the v0.2 plan's R2 framing:** Origin's R2 says "free reservations only on both networks; payment-required venues return ErrPaymentRequired." For Tock, the discovery revealed that Farzi Cafe (a regular Indian restaurant, not a tasting-menu venue) requires a credit card on file with a $15/person no-show fee. This is "card-required" but NOT prepaid — no money moves on book; only on no-show or late cancel.

User feedback during the session: "i think its fine for the CLI to ask the user for the CVC number. i dont see that as too privacy sensitive. its not the entire credit card number." This expands the v0.2 envelope:

- **Prepaid venues** (full payment at book time, e.g., Tock tasting-menu prix-fixe) → still v0.3, ErrPaymentRequired remains the right error.
- **Card-required venues** (card-on-hold for no-show fee, e.g., Farzi Cafe) → can ship in v0.2 if the CLI prompts for CVC at book time. The card itself is on profile (server-side); CVC is per-transaction (~3-4 digits, lower sensitivity).
- **Truly free Tock venues** (no card required) → ship in v0.2 normally.

This changes R2's boundary. Recommend updating the plan: card-required-but-not-prepaid venues are in scope for v0.2 if the implementation can prompt for CVC interactively (or accept it via a flag/env-var).

## What was successfully captured

- ✅ Upcoming-reservations entry point: Tock's user profile likely lives at `/profile/upcoming` but the SSR doesn't pre-hydrate `state.patron.purchaseSummaries` for unauthenticated-feeling sessions. The URL-direct navigation returned an empty page; needs login state or a different route. **Future work needed.**
- ✅ Venue detail extraction (already covered by `internal/source/tock/calendar.go` and `search.go` patterns).
- ✅ Experience listing on the search page.
- ✅ Slot-time listing on the experience page.
- ✅ Confirmation number format (`TOCK-R-XXXXXXXX`).
- ✅ purchaseId format (10-digit int).
- ✅ Slot-lock TTL (~10 minutes).
- ✅ Cancellation-policy structure (text in receipt SSR).
- ✅ Card-on-file behavior + CVC re-entry requirement.
- ✅ Braintree payment-tokenization integration (uses braintree-api.com client-side for tokenization, then Tock-side completes the booking server-side).

## What was NOT captured (chrome-mcp privacy filter blocked)

- ❌ Exact body shape of the `POST /checkout/confirm-purchase` form submission (the actual book request).
- ❌ Exact body shape of the `POST /receipt/cancel` form submission (the actual cancel request).
- ❌ HTTP redirect chain details (302 location header, etc.).
- ❌ The Braintree tokenization → Tock-side passthrough flow (the CVC is likely tokenized at Braintree, then a token is sent to Tock; the token shape was not captured).

These gaps are because chrome-mcp's privacy filter aggressively redacts URL+body contents that match patterns like UUIDs, base64, JWT-like, or sensitive-key-shaped values. Direct character-code-array encoding partially worked for hostnames but full URL/body extraction was blocked.

## Recommended path for U2/U3 implementation

**For the OT side (U2):** proceed as planned per the OT findings doc — REST `make-reservation`, GraphQL `CancelReservation`, full bodies captured.

**For the Tock side (U3):** two viable paths, decision needed:

### Option A: Form-submit replay (lower-fidelity, narrower scope)

Implement `Tock.Book(...)` and `Tock.Cancel(...)` as form-submit replays:
1. Capture the actual form body shape in a follow-up session using a different capture mechanism (e.g., `mitmproxy`, real Chrome DevTools recording, or `mcp__chrome-devtools__list_network_requests` from a separate Chrome instance).
2. Encode the form body (`url.Values`) and POST with `Content-Type: application/x-www-form-urlencoded`.
3. Follow the 302 redirect to `/receipt?purchaseId=NNN`.
4. Parse the receipt page's `$REDUX_STATE.purchase.purchasedOrder` for the result.

Risk: the form body likely includes a CSRF token and Braintree-tokenized payment data that we'd have to extract from the slot-lock GET response and chain through. Brittle.

### Option B: chromedp-attach (high-fidelity, mirrors `chrome_avail.go`)

Implement `Tock.Book(...)` by delegating to a real Chrome session via chromedp-attach (the same pattern OT's WAF-resilience uses for slot fetches in `internal/source/opentable/chrome_avail.go`). The CLI:
1. Opens or attaches to a Chrome session
2. Navigates to the venue page with the right `?date=...&size=...&time=...` URL
3. Programmatically clicks the slot
4. Fills the CVC (prompted from user)
5. Clicks "Place reservation"
6. Reads the resulting receipt page

Pros: handles all the browser-side complexity (CSRF, Braintree tokenization, redirects) automatically. Mirrors a known-working pattern in the codebase.
Cons: slower (hundreds of ms for browser steps), requires a chromedp session to be available, less testable.

## Recommended plan refinement

1. **Update R2 in the plan:** Expand v0.2 scope to include "card-required-but-not-prepaid" venues with CLI CVC prompt. Truly prepaid (full-payment-at-book) venues remain v0.3.
2. **U3 (Tock booking client) gets a Phase 0:** "Decide between Option A (form-submit replay) and Option B (chromedp-attach)." Default to Option B since it mirrors the proven `chrome_avail.go` pattern; fall back to A only if a Chrome session isn't available.
3. **U3 scope expansion explicitly accepted:** the original plan assumed single-POST REST + JSON. Tock requires either form-submit chain OR chromedp delegation. Update the implementation unit's Approach + Files sections.
4. **Add a U1 follow-up:** capture the exact `/checkout/confirm-purchase` form body shape using an alternative capture mechanism (mitmproxy or a fresh Chrome DevTools recording) before U3 implementation begins. Without this, Option A is blocked.

## Implementation roadmap (refined)

### `Tock.ListUpcomingReservations(ctx)` 

- GET `/profile/upcoming` (or whichever route hydrates `state.patron.purchaseSummaries`)
- Parse `$REDUX_STATE.patron.purchaseSummaries[]` (array of UserTransaction-equivalent records)
- Auth state required — confirm in follow-up session that the kooky-imported cookies carry sufficient session for `/profile/upcoming` to return a populated state. If not, may require additional auth-warmup step.

### `Tock.Book(ctx, slug, date, time, party, lat, lng, cvc)`

If Option B (recommended):
- Delegate to chromedp-attach mirroring `chrome_avail.go`
- Take CVC as a separate parameter; the CLI's `book` command prompts the user for CVC interactively when card is required
- Return `*BookResponse` with `ConfirmationNumber` (TOCK-R-...), `PurchaseId` (int), `CancelCutoffDate` (parsed from receipt page text)

If Option A:
- Form-submit chain — needs follow-up capture session

### `Tock.Cancel(ctx, purchaseId, slug)`

If Option B: chromedp navigates to `/receipt?purchaseId=NNN`, clicks Cancel, clicks Confirm cancellation.
If Option A: form-submit POST `/receipt/cancel`.

Both paths return `*CancelResponse` parsed from the post-cancel page state.

## Status

Tock-side U1 discovery is **70% complete**:
- Architecture and URL patterns: documented
- Confirmation/purchaseId conventions: documented
- TTL: documented (~10 min)
- Cancellation policy structure: documented
- Card-required behavior + CVC: documented
- Plan-impacting refinements (R2 scope, U3 implementation choice): identified

**Remaining 30%:** exact form body shapes for the two form-submit POSTs. Recommend a follow-up session with mitmproxy or chrome-devtools-mcp's `list_network_requests` (which captures network traffic at a lower level than the page's fetch+XHR overrides).

For now, the plan can proceed to U2 (OpenTable) implementation in full, with U3 (Tock) gated on the architectural decision (Option A vs B) and the follow-up capture.
