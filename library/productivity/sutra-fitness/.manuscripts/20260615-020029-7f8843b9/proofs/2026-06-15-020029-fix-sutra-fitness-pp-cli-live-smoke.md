# Sutra Fitness CLI — Live Smoke Test (Phase 5, full)

Run against the real Sutra/Arketa Partner API with operator-provided credentials
(read-only). Credentials never written to any artifact.

## Reachability + auth
- `GET /{partnerId}/locations` with `X-API-Key` → 200, documented envelope. Auth model confirmed.

## Live sync (the foundation)
- `sync` pulled the FULL dataset: locations 16, classes 865, clients 6995,
  purchases 4461, reservations 22143, rooms 40, referrals 0 — **34,523 records,
  7/7 resources, 0 errors**.
- Dependent sync verified live: reservations fetched per-class (22,143 across 865
  classes), rooms per-location (40 across 16 locations).

## Two critical bugs found by live testing and fixed
1. **Cursor double-encoding** — `pagination.nextStartAfterId` arrives `%2F`-encoded;
   the client's `url.Values.Encode()` re-encoded it to `%252F`, so the API returned
   an empty page 2 and sync silently capped at the first page (10 rows/resource).
   Fixed with `decodeSutraCursor` (URL-decode before the client re-encodes) applied
   in the generic sync loop and the dependent fetch. Also fixed the empty
   `resourceSupportsPagination`/`determinePaginationDefaults`/registry the generator
   left blank. Result: 10 → 6,995 clients, etc.
2. **Status enum mismatch** — the spec declares reservation statuses
   `CHECKED_IN`/`NO_SHOW`, but real data uses `ATTENDED` (= `checked_in` flag),
   `CANCELLED`, `WAITLISTED`, and has **no** `NO_SHOW`. scorecard/no-shows hardcoded
   the spec values, so check-in/no-show rates were always 0. Fixed: attendance =
   `ATTENDED`/`checked_in=1`; no-show = a `BOOKED` reservation for a class that has
   already started. churn/referral-funnel already used the `checked_in` flag.

## All 8 analytics verified on real data (PII redacted)
- `scorecard` — real fill / attended / no-show rates per instructor (e.g. 717 attended / 534 no-shows = 42.69%).
- `no-shows` — by instructor/class/client, real rates.
- `utilization` — by class/instructor/timeslot/location (note: aggregate fill skewed by sentinel max_capacity values in source data; per-instructor view is clean).
- `expiring` — 69 plans within 14d incl. low-credit flagging.
- `churn` — 6,743 at-risk clients at 60-day inactivity.
- `revenue` — $10,490 total by type with prior-period deltas (+180% pack, +81% subscription).
- `referral-funnel` — funnel counts (0 referrals for this partner).
- `ltv` — clients ranked by lifetime spend ($1,914 top) with tenure.

## Gate: PASS
All core workflows exercised against the live API; both discovered bugs fixed,
re-verified, and covered by unit tests (incl. real no-show semantics).
