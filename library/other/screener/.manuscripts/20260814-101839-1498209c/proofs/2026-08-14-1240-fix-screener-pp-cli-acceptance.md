# Screener.pp CLI Live Dogfood Acceptance Report

## Level: Full Dogfood (live, authenticated session)

## Result: 197/203 passed (97%)

## Failures (6, all rate-limit artifacts)
- results latest [happy_path, json_fidelity] — HTTP 429 from screener.in during matrix burst
- screens list [happy_path, json_fidelity] — HTTP 429 during matrix burst
- screens run [happy_path, json_fidelity] — HTTP 429 during matrix burst

All 6 failures are `exit 5` (rate limited). Screener.in throttles burst access at ~1 req/s. The live-dogfood matrix fires each test in a fresh subprocess at full speed; the generated client's retry is intentionally disabled under PRINTING_PRESS_DOGFOOD=1 (generator safety design), so a 429 during the matrix fails immediately.

**Verified NOT a CLI defect:** every command succeeds with normal cadence:
- 6 rapid-fire `screens run` in normal mode (retries enabled): 6/6 exit 0
- All 16 commands with human pacing: 16/16 pass
- The CLI surfaces 429s as typed rate-limit errors with Retry-After guidance

## Fixes applied during dogfood
1. **profile-standalone 404** — Screener.in moved standalone to the base URL `/company/{symbol}/` (no suffix). Spec + command fixed.
2. **market example misleading** — help showed `market sector IN08/...`; the command takes `<sector_path>` directly. Example fixed to `market IN08/IN0801/IN080101`.
3. **insider-flow zero values** — trades page is `<tr>`-based with field-classed `<td>` cells, not div blocks. Parser rewritten to parse table rows; now returns real net buy/sell flows (THYROCARE +65,200 lacs, METROPOLIS -11,844 lacs).
4. **feedback --help missing Examples** — added Example block.
5. **feedback error_path false-fail** — any text is valid feedback; added pp:no-error-path-probe annotation.
6. **Default rate-limit 1 req/s** — the CLI now paces politely by default (screener.in throttles bursts).
7. **Novel commands retry once on 429** — getWithRateRetry helper (Retry-After aware).

## Auth
- Authenticated via session cookie imported from the user's Chrome session (auth login --cookies-file, in-memory only)
- Auth-gated commands verified live: results latest, trades insiders, filings, insider-flow, full-text-search

## Gate: PASS (with documented rate-limit artifacts)
The 6 rate-limit failures are environmental (matrix cadence vs site throttle), not functional defects. All features verified working live.

## Final Dogfood Status (post-fixes)

- 198/203 tests pass (97.5%) on the best run; failures fluctuate 1-7 depending on the site's rolling throttle window.
- All failures are exit 5 (HTTP 429) from screener.in's anti-burst rate limiter reacting to the matrix's ~50 rapid subprocess calls.
- **Verified NOT a CLI defect:**
  - 16/16 commands pass under dogfood env with 1.5s spacing
  - All 5 novel-feature live probes pass (5/5)
  - Rapid-fire isolated calls all pass
  - The CLI's retry (3x with Retry-After pacing) clears individual throttles; the matrix's cumulative burst exceeds the site's window
- **Known environmental gap (user-approved):** the dogfood matrix's burst cadence trips screener.in's rate limiter on 3-5 commands. Promotion proceeds with this documented gap. The CLI correctly surfaces 429s as typed rate-limit errors with Retry-After guidance.
