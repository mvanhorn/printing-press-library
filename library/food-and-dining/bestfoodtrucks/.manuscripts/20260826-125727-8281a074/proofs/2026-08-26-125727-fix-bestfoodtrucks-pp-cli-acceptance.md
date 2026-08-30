# Acceptance Report: bestfoodtrucks

- Level: Full Dogfood
- Tests: 98/98 passed (100%)
- Skipped: 89 (structurally unverifiable in this harness shape — help/dry-run/JSON-fidelity checks for framework commands with no meaningful live-data assertion, e.g. `profile`/`auth`/`teach` subcommands)
- Failures: 0 (after one inline fix)
- Fixes applied: 1
  - `feedback` command's generator-emitted `--help` text was missing an `Examples:` block, failing the mechanical help-check. Added a real example block (matching the existing convention in the same file). Rebuilt, re-ran dogfood — 98/98 (100%).
- Printing Press issues (for retro): 0 new in this pass (the `which_test.go` positional-vs-path-segment gap and the Phase-4→Phase-5 `live_api_verification` sequencing dependency were already logged in the build log / shipcheck report from earlier phases).
- Gate: **PASS**

## Redacted evidence

No PII appears in any live response for this API — all data (lots, trucks, shifts, menu items, market rosters) is public business/location information, not personal data. No redaction was necessary in this report.

## Independent verification beyond the matrix

Throughout Phases 3-5, in addition to the binary-owned matrix, the following commands were manually verified against real live data with arguments *not* used anywhere in delegation prompts or research.json examples, confirming genuine parameterization (not hardcoding):
- `lot digest at-t-los-angeles`, `lot digest lacma`
- `truck schedule 13` (StopBye #1 — a truck with historical data back to 2017)
- `market hotlist atlanta`, `market hotlist boston`, `market list boston`
- `market hotlist los-angeles --limit 5` (100-truck fan-out, timed at 3.2s after the rate-limiter fix)

All returned correct, plausible, verifiably-real data distinct from every other tested input.
