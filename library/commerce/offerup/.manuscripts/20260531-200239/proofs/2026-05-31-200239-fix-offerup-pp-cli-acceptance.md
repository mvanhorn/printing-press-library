# OfferUp CLI — Phase 5 Live Dogfood Acceptance

**Run:** 20260531-200239 · **Level:** Full Dogfood · **API auth:** none (public, no key)

## Result: PASS
- Binary-owned live matrix: **44/44 passed** (`phase5-acceptance.json` status `pass`).
- Hit live OfferUp (unauthenticated, rate-limited 2 rps). No write side-effects — OfferUp reads are public; this CLI has no mutating/auth commands.

## Authenticated flows: none by design
The CLI is entirely unauthenticated per the user directive ("prefer unauthenticated; only require auth when required") and the absorb-gate decision. Auth-gated OfferUp features (saved items, messages, my-listings, posting, offers) were deferred to out-of-v1-scope and are not built. All 44 matrix tests exercise unauthenticated paths. There are no authenticated flows to test.

## Fix applied during dogfood
- **seller-scan error-path** (1 failure → fixed): the error-path probe expected a non-zero exit for an invalid arg, but `seller-scan <unknown-id>` reads the local store and legitimately returns exit 0 + empty inventory + a "run `listings get` first" hint — an unknown id is indistinguishable from a valid-but-unsynced seller. Added `cmd.Annotations["pp:no-error-path-probe"] = "true"` (the documented opt-out for this exact case). Re-ran: 44/44 pass.

## Coverage
- Every leaf command: help, happy-path, JSON-fidelity, error-path (where applicable).
- Novel features behaviorally re-confirmed live in Phase 3/4: price-check (median over real listings), deals (ranked below-median), new-since (new-listing diff), price-drops (honest empty on first run), seller-scan (store-read), digest (composite), listings search (clean, ad-filtered), listings get (full detail + seller + condition).

## Gate: PASS → proceed to Phase 5.5 Polish.
