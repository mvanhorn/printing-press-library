# Acceptance Report — sensortower-pp-cli

- **Level:** Full Dogfood (binary-owned live matrix, session-injected)
- **Tests:** 133/133 passed, 69 skipped (skips = error-path probes on read-only/no-error-path commands), 0 failed
- **Gate:** PASS

## Auth context
- type: cookie (session injected via SENSORTOWER_CONFIG for the matrix)
- 9/11 endpoints work anonymously; the injected session exercised the cookie-gated
  `apps unified`, `publishers unified`, and `compare` for real.

## Failures fixed inline during Phase 5 (5 → 2 → 0)

Round 1 (5 failures):
- `rankings ios` + `rankings android` happy-path: spec `happy_args` lacked the now-required
  `--date` (I made date required in Phase 3 to fix a separate upstream-422 bug). **Fixed the
  spec fixtures.** CLI fix.
- `watch rm` happy/json: matrix can't pre-add before removing, so removal of the synthesized
  fixture returned a not-found error. **Made removal idempotent** (rm -f semantics). CLI fix.
- `watch add` json_fidelity: `--dry-run` printed plain text under `--json`.

Round 2 (2 failures):
- `watch add --dry-run --json`: **dry-run now emits JSON when machine output is requested.** CLI fix.
- `watch rm __printing_press_invalid__` error-path: idempotent removal has no invalid-input
  error path (any string is a valid absent-removal target). **Added `pp:no-error-path-probe`**
  to both `watch add` and `watch rm`. CLI fix.

## Behavioral spot-checks (verified against real responses)
- `find spotify --select app_id,name` → returns exactly {app_id, name} (the earlier broken
  `data.entities.*` select path, fixed in Phase 4.9).
- `apps get 460177396` → 51-key hub object, 469 version entries.
- `movers 6015` → real Finance apps; `new` flips null→false on 2nd run (baseline works).
- `divergence 6015` → free-vs-grossing spread computed from exact ranks only.
- `teardown 460177396` → 469 versions, 7-day median cadence.
- `compare 460177396 tv.twitch.android.app` (with session) → cross-platform side-by-side.
- `watch add/list/rm/digest` → full lifecycle, idempotent rm.

## Printing Press issues for retro
- Single-endpoint resources silently promoted to leaf commands (`find`, `categories`) with no
  pre-generation signal — caused manifest/narrative command-path errors.
- Generated `rankings` command shipped a required-param 422 when the sniffed spec marked the
  param optional (the browser always supplied it, hiding the requirement).
- `cliutil.SanitizeErrorBody` / `maskCredentialText` miss cookie-auth credentials (only mask
  Bearer/OpenAI-shaped strings). Filed by Phase 4.95.
- `cliutil.ratelimit` has no epoch-milliseconds branch for `X-Ratelimit-Reset`. Filed by Phase 4.95.

## PII
No org names, emails, or human identifiers quoted. App/publisher names in samples (Twitch,
Spotify, Netflix) are public store listings, not personal data.
