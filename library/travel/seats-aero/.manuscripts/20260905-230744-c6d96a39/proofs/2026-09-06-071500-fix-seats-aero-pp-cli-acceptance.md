# seats-aero reprint — Phase 5 acceptance (live dogfood, 2026-09-06)

```
Acceptance Report: seats-aero
  Level: Full Dogfood (user-selected)
  Runner: cli-printing-press 4.31.7 `dogfood --live --level full --write-acceptance`
  Isolation: XDG_*_HOME → pipeline/xdg/*, SEATS_AERO_NO_AUTO_REFRESH=1 (operator's live store parked; never touched)
  Tests: 101/101 passed, 0 failed, 93 skipped (structural), coverage_hollow: none
  Runs: 4 (see below) — final marker written by the runner: proofs/phase5-acceptance.json status=pass
  Auth context: api_key (SEATS_AERO_API_KEY, read-only GETs; /live commercial-tier and /refresh credit-tier skipped by annotation)
  Gate: PASS
```

## Run history
| # | Result | Why it was re-run |
|---|---|---|
| 1 | PASS 101/101, **hollow: recheck** | recheck classified mutating → happy path ran only as `--dry-run` |
| 2 | PASS, hollow: recheck | `pp:method: GET` added but the matrix ran a STALE binary (built before the edit) |
| 3 | PASS, hollow: recheck | annotations compiled in (`mcp:read-only=false`, `pp:method=GET`, `pp:typed-exit-codes=0,3`) — still `--dry-run`: the runner takes the happy path from the Go `Example` string, which still carried `--dry-run` |
| 4 | **PASS, no hollow features** | `Example` switched to the print-only default; recheck happy path ran the real plan (mode `plan`, one quota probe) |

## Skips (93) — all structural, none are novel-feature happy paths
- 26 `no positional argument` (error-path probes for flag-only commands)
- 11/9/5/3 non-id positionals (`name`/`query`/`resource`/`text`) at depth 0 — framework commands (`teach`, `recall`, `search`, …)
- 10 `mutating command dry-run only`; 9 `mutating command; error_path would call live API without --dry-run`
- 4 `destructive-at-auth`
- `live` skipped by `pp:requires-tier: commercial`; `refresh` by `pp:requires-tier: refresh-credits` (by design)

## Novel-feature happy paths (all live, all PASS)
- `new-since` → `results: []` on the empty isolated store + stderr sync hint (correct empty contract)
- `calendar`, `direct-scan` → `[]` (same)
- `reach --origin JFK --cabin business --top 3` → live `/destinations`, 3 ranked destinations, `local_evidence: null`, `live_check: null`
- `recheck … --older-than 1h` → plan envelope: `mode: plan`, shortlist `[]`, `would_refresh: 0`, quota block from the live probe

## Fixes applied during Phase 5 (all committed in the working tree)
- `recheck` annotations: `mcp:read-only=false`, `pp:method=GET`, `pp:typed-exit-codes=0,3` (splitwise print-only live-gate parity) — CLI fix
- `recheck` `Example` without `--dry-run` — CLI fix
- Printing Press issues for retro: the runner's `Binary refresh` did not rebuild a binary older than the sources on run 2; the happy path is derived from `Example`, so a `--dry-run` example silently hollows a feature; `verify --env-var` does not enable live mode.

## PII
Live output samples contain route/airport/mileage data only; no account identifiers, names, or tokens. The raw results JSON (`*-dogfood-results.json`) stays in the private manuscripts; per workshop policy it must not be embedded in a public publish package without review.
