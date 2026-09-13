# immoweb-pp-cli shipcheck

## Final run (shipcheck --api-key public: live read-only verify; auth type none so the value is never sent)
```
Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            PASS    0         19.769s
  validate-narrative  PASS    0         274ms
  dogfood           PASS    0         4.147s
  workflow-verify   PASS    0         20ms
  apify-audit       PASS    0         43ms
  verify-skill      PASS    0         5.14s
  scorecard         PASS    0         7.94s

Verdict: PASS (7/7 legs passed)
  Live API Verification   10/10
  Total: 89/100 - Grade A
  Passed: 5/5  (100% pass rate, 0 skipped)
```

## Loops
1. Run 1 — FAIL (verify-skill) + HOLD: verify-skill flagged a troubleshoot line pairing `locations` with `--postcode`; live probe: market/yield timeouts, deal SIGBUS, triage "duplicate column old_price". Scorecard 85.
2. Fixes: reworded troubleshoot (research.json + README/SKILL), fixed concurrent schema race (old_price in CREATE, tolerate duplicate ALTER), store hardening no longer opens/closes DB files mid-transaction (POSIX lock release -> corruption; recorded in .printing-press-patches/), harness page curtailment.
3. Run 2 — HOLD: probes market/yield timed out (cold harvest), triage probe had no saved search.
4. Fixes: triage ad-hoc filter mode; price-band harvest through the map endpoint (complete area in ~25 light requests: Ixelles rentals 1,361/1,361; the earlier 200-listing map sample was biased +21% on the median); harvest rate ceiling 8 req/s for harvesting commands (generic auto start is 2 req/s); adaptive initial bands.
5. Run 3 — HOLD only on live_api_verification (mock verify). Run 4 with live read-only verify — PASS 7/7, scorecard 89/100 Grade A, live probes 5/5.

## Before / after
- verify: PASS (mock) -> PASS (live)
- scorecard: 85 -> 89 (Grade A)
- live sample probes: 1/5 -> 5/5

## Remaining scorecard gaps (non-blocking)
- MCP token efficiency 4/10, insight 4/10, cache freshness 5/10, dead code 2/5 (generator-emitted dead helpers handleBinaryResponseDelivery, readSecretFromStdin, successfulNoop).

## Verdict: ship
