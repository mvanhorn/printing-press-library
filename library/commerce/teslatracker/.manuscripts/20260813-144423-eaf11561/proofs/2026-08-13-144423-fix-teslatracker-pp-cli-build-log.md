Manifest transcendence rows: 10 planned, 10 built. Plus `hydrate` (new, required by all of them).

## Design corrections made during Phase 3
- `priceHistory` is served by the API (snapshots ~every 2h), not only accumulated locally.
  `price-history` and `stale` therefore work on first sync instead of after weeks.
- Local price snapshots are RETAINED anyway: a departed listing takes its server history with
  it, and `gone` needs the price path of cars that are no longer listed. `coverage` reports
  divergence between local and server history.

## Built so far
- `hydrate` (new, not in the manifest) — sync stores links; hydrate turns each VIN into a
  full vehicle record. Every derived command depends on it. 23/23 hydrated, 0 failures.
- `warranty <vin>` — four limits, projected to a delivery date at the user's annual mileage.
- `degradation [vin]` — rated-vs-actual retained %, cohort percentile, or the whole curve.
- `comps <vin>` — landed-cost cohort placement with n, median, gap, and the formula printed.

## Bug found by running it (fixed)
`warranty` picked the arithmetically-smallest limit as "binding". On a 2018 car that was
`vehicle (time)` at -46.8 months — expired in 2022, unremarkable, and it buried the live
constraint: battery cover expiring in 1.2 months. Expired limits are now reported separately
under `already_expired` and only unexpired limits compete to be binding.

## Still to build (7)
gone · price-history · premium · stale · radius · watch · coverage
