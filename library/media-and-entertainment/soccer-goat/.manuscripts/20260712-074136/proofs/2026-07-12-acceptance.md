# soccer-goat Phase 5 Acceptance

Level: Full Dogfood (live)
Gate: PASS — 104/104 tests (matrix_size 104, 0 failed)

## Live behavioral verification (real data)
- player schjelderup -> €30.00m, EA 72, LW, age 22, Norway, SL Benfica (matches user's example)
- player mbappe -> €180.00m, EA 91, Real Madrid
- team benfica -> 30 players, squad value €384.00m, sorted by value with ratings
- compare mbappe haaland -> €180m/91 vs €200m/91
- over-under-rated benfica -> market-hyped (Schjelderup €30m/72) vs bargains (Bruma €3m/77), 9 skipped
- wonderkids benfica --max-age 21 -> 5-6 young players ranked
- Potential uniformly best-effort unavailable (Cloudflare) — graceful, no errors, as designed

## Fixes applied during dogfood (all CLI-scope, none block ship)
1. Potential client skips network without clearance cookie (team 19.9s -> 3.7s).
2. EA limiter 3->8 req/sec for squad-wide enrichment under probe budget.
3. Framework teach/playbook commands: added pp:happy-args fixtures so dogfood can exercise them.
4. teach: emit JSON on explicit --json (machine-output request) while default stays silent (background contract).
5. teach Example: removed shell line-continuation backslashes that leaked into dogfood's parsed arg vector.

Gate: PASS
