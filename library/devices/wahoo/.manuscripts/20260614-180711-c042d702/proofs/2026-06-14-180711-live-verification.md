# Wahoo CLI — Live Verification (publish-time)

Live-tested against a real, consented Wahoo account (sandbox OAuth app, read-only token).
Personal ride statistics are intentionally omitted from this public artifact.

## Result
- OAuth2 confidential Authorization-Code login: success; `doctor` reports auth valid + API reachable.
- `sync`: pulled the account's full workout history (hundreds of rides) into the local store.
- `digest` / `bests` / `load`: produced correct, sane values (training-load CTL/ATL/TSB, records, windowed rollups).
- `backup`: downloaded real FIT files from the Wahoo CDN (`cdn.wahooligan.com`, unauthenticated/public URLs) — 4/4 in a recent-window sample, 0 failures.
- Official live gate: `dogfood --live --level quick` → 17/17 PASS (read-only; full write-lifecycle deliberately not run against the real account).

## Bugs caught by live data (and fixed) — spec-vs-reality gaps
1. `work_accum` is reported in **joules**, not kilojoules (the OpenAPI spec was unit-less). Fixed: convert to kJ (was ~1000x over-reported, which also inflated the load estimate).
2. Average power is field **`power_avg`** in real responses, not the spec's `power_bike_avg`. Fixed: read `power_avg` (spec name kept as fallback). Previously all power analysis was silently empty.
3. Real responses also carry **`power_bike_np_last`** (normalized power) and **`power_bike_tss_last`** (per-ride TSS, present on most rides). `load` now uses Wahoo's recorded TSS instead of an estimate.

## Known minor issue
- Syncing the `user` profile errors: `/v1/user` is a singleton, which the list-oriented syncer mishandles (non-fatal warning; `user get` works directly).
