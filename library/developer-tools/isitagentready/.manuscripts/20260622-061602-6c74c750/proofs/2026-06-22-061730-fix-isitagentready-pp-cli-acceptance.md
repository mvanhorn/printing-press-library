# Phase 5 Acceptance Report — isitagentready-pp-cli

- **Level:** Full Dogfood (live, against the real no-auth API)
- **Result:** PASS — 38/38 ran-tests passed, 0 failed, 48 skipped (matrix 38 executed)
- **Auth context:** none (public, no-auth API; live testing has no cost/side effects)
- **Gate marker:** `proofs/phase5-acceptance.json` (status: pass)

## What ran
Binary-owned live matrix (`dogfood --live --level full`): help, happy-path, JSON-fidelity, and
error-path checks across every leaf command, executed against `POST /api/scan` for real.

## Failures found and fixed (first run: 8/46)
All 8 first-run failures were `error_path` probes that passed a deliberately-invalid argument
expecting a non-zero exit. Root cause: the scan API returns **HTTP 200 + a `siteError` block** for
any unreachable/garbage URL (it never 4xx's on a bad target), so `check`/`report`/`gate`/`compare`/
`diff`/`history` correctly exit 0 (the scan succeeded; the target site errored), and `batch`/`guide`
degrade gracefully under dogfood. These commands cannot distinguish bad input from a valid
empty/siteError result without inventing API-specific semantics.

**Fix (CLI):** annotated the 8 commands with `pp:no-error-path-probe: "true"` (the documented
mechanism for exactly this case). Real-user error paths are unchanged: `advice` still errors on a
siteError (exit 5), `batch` still errors on a genuinely missing file (exit 2) outside dogfood, and
`guide` still returns notFound (exit 3) for an unknown check outside dogfood.

**Printing Press issues for retro:** none from dogfood. (Separate retro note: the generator emits a
no-store scaffold + novel-command stubs for stateless POST-only APIs, and emits unused partial-failure
helpers — see shipcheck/4.95 notes.)

## Manual spot-checks (live)
check, advice (+--copy), report (+--only-failing/--select), guide (fetches real SKILL.md), gate
(exit 0 at/above min, exit 1 below), diff, history, compare (21-check matrix, ~6.6s parallel),
open-advice (cross-site backlog) — all produce correct output.

## Gate: PASS → proceed to Phase 5.5 Polish.
