# Pacing and no-retry proof, offline (2026-10-05T13:2xZ)

Zero Uber traffic: every request went to a loopback fake (scratchpad fakeuber, built from discovery/contract/out-01). Each line below is a SEPARATE CLI process, so the gap is the cross-process gate (state-dir lock plus last-request file), not an in-process sleep. The CLI wrote UBER_JOBS_REQUEST_LOG; the fake logged arrival times with milliseconds.

## Run 1, gate at 3.0 s (before the fix)
- 6 processes, 12 requests: postings GBR, postings DEU, stats, check, facets, get (all live, --no-cache).
- Server-side gaps (s): 2.998, 3.003, 2.999, 2.999, 3.023, 2.980, 3.023, 2.979, 3.002, 3.017, 2.984. Min 2.979 s: send jitter put arrivals about 20 ms under 3 s. The owner rule is "at least 3 s apart", so the gate was raised.
- Refusal: the site answered a challenge. Process 1 sent ONE request (403) and wrote the latch refused-<host>.json. Processes 2 and 3 sent ZERO requests to the site (latched) and went straight to the Oracle fallback (a dead loopback port here, so they exited 7). The fake received exactly 1 refused request.

## Run 2, gate at 3.5 s (MinRequestGap = 3 s floor + 0.5 s jitter margin)
- 4 processes, 8 requests: postings GBR, postings DEU, stats, get.
- Server-side gaps (s): 3.501, 3.501, 3.502, 3.500, 3.523, 3.479, 3.520. Min 3.479 s.
- CLI request log (second resolution, the ledger format of rule 14) gaps: 4, 3, 4, 3, 4, 3, 4. Min 3 s.

## Still to do before the first press live leg
Prove the same on discovery/uber-request-ledger.tsv from the first real live leg: merge the CLI request log, then check every consecutive uber.com/oraclecloud.com pair is >= 3 s apart and that no request follows a 403/429/challenge from the same host.
