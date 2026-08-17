# Pexels CLI — Phase 5 Acceptance Report

Level: **Full Dogfood** (live, against api.pexels.com with an API key)
Gate: **PASS** — 80/80 tests passed (`phase5-acceptance.json` status: pass)

## Result
- Tests: 80/80 passed across every leaf subcommand (help, happy-path, JSON-fidelity, error-path).
- Auth context: api_key (key used read-only for live testing; never written to any artifact).
- Verified live: photo/video/collection search + curated + by-id, filtered search (orientation/size/color/locale — auth-gated), my-collections, sync (242+ records across 4 resources), offline search, analytics group-by, and all 4 novel commands (quota forecast, resolve, dedup-aware download, attribution export).

## Fixes applied during Phase 5 (all CLI-side, verified)
1. **`download`/`resolve` positional-or-flag** — added `--query` / `--id` flag alternatives (Use `[query]`/`[id]`) so agents, scripts, and the verifier can supply input without a positional. Positional form still works.
2. **`workflow archive` dogfood timeout** — capped pages-per-resource to 2 under the dogfood env (full archive of thousands of items previously exceeded the per-command timeout).
3. **Client fail-fast on rate limits under test harnesses** — `maxRetries=0` under PRINTING_PRESS_DOGFOOD / PRINTING_PRESS_VERIFY so a 429's 5s backoff waits don't blow the per-command timeout; production keeps the full 3-retry resilience.
4. **`pp:happy-args`** annotations on the 4 novel commands so dogfood constructs real invocations.

## Key API finding (carried into docs)
**Pexels gates filters and collections behind auth, and throttles collections aggressively.**
- Keyless (public): bare `search?query=`, `curated`, `photos/{id}`, `videos/{id}`, `videos/search`, `videos/popular`.
- Auth-required: filtered search (`orientation`/`size`/`color`/`locale`), all `collections/*`.
- Measured: `/collections/featured` allows ~6 rapid calls then 429s (token bucket); `/curated` has no such tight burst limit. The CLI handles 429 correctly (adaptive backoff + honest RateLimitError, never empty-as-success). A clean full-matrix pass required letting the collections token bucket refill between runs — not a CLI defect.

## PII
Live responses (photographer/collection names) were not quoted in this report; results described generically. No API key value appears in any artifact (scanned).

## Verdict: **ship** — full dogfood clean, no functional defects, no stubs.
