# HotelTonight CLI — Acceptance Report

Level: **Full Dogfood** (binary-owned live matrix, real API, no auth)
Matrix: 64 mandatory tests — **59 passed, 5 failed**
Runner marker: `phase5-acceptance.json` status = `fail`

## The 5 failures — all matrix-fixture / generated-command artifacts, not shipping-scope bugs

| Command | Kind | Cause | Real-world? |
|---------|------|-------|-------------|
| `inventory --latitude example-value --longitude example-value` | happy_path | Matrix passes the literal placeholder `example-value` as coordinates; API returns 400 for non-numeric lat/lng. | Works with real coords (verified). Matrix can't synthesize valid geo. |
| `inventory ... --json` | json_fidelity | Same — non-numeric coords → 400. | Same. |
| `history __printing_press_invalid__` | error_path | Returns empty + "no history yet, run deals/watch first" at exit 0. Matrix expects non-zero. | Correct: name-substring lookup, "no match → empty" is a valid answer, not an error. |
| `verdict __printing_press_invalid__` | error_path | Returns `verdict: unknown` + guidance at exit 0. | Same as history. |
| `workflow archive --json` | output_mismatch | Generated framework command streams NDJSON sync events; probe expects single JSON. | Generated behavior, not novel code. |

## Every shipping-scope feature passes with real input (re-verified live)
- Base: `markets list/get/nearby`, `inventory` (real coords) — correct data, header-only.
- Novel: `deals` (+ `--category` tier filter), `watch`, `history`, `verdict`, `compare-neighborhoods`, `datescan`, `daily-drop` (incl. the hidden Daily Drop reveal) — all return correct, relevant output.
- `--json`, `--select`, `--agent` produce clean structured output on every command.

## Assessment
Real verdict: **ship**. No flagship or approved feature is broken. The 5 matrix failures are synthetic-input probes (invalid placeholder coordinates), correct empty-result behavior misread as errors, or generated-framework streaming output.

## Retro candidates (machine, not this CLI)
1. **Live-dogfood / scorecard fixture synthesis for geo-typed flags** — passing `example-value` to numeric lat/lng flags guarantees a 400 on any geo API. The matrix should synthesize plausible numeric coordinates (or skip happy-path) for flags named latitude/longitude/lat/lng.
2. **Generated endpoint-mirror error passthrough** — the promoted `inventory` command surfaces the raw upstream 400 (`ExpectedMobileApiException`) verbatim; the generator could validate numeric path/query params and emit an actionable error.
3. **Staged-binary staleness** — live-check resolves `build/stage/bin/<name>` first, which is stale after Phase 3 hand-coding until manually rebuilt.

## Hardening added (user-requested)
Added numeric coordinate validation to the shared `resolveGeo` (used by `deals`, `watch`, `compare-neighborhoods`, `datescan`, `daily-drop`): non-numeric or out-of-range coords, and one-coord-missing, now produce an actionable error (`--lat must be a number (got "example-value"); e.g. --lat 37.7749 --lng -122.4194, or use --metro <id>`) at exit 1 *before* any API call — instead of a raw upstream 400. Verified: bad coords → exit 1 with the message, valid coords → exit 0, `history`/`verdict` no-data → exit 0 (empty is valid). Covered by `TestValidateCoord` / `TestResolveGeoCoordValidation`.

The 2 `inventory` matrix failures persist and are now fully explained: the matrix can satisfy `deals` via its `--metro` integer flag (so `deals` passes), but the **generated** `inventory` mirror's only geo input is its string `--latitude`/`--longitude` flags, which the matrix fills with the literal `example-value` → 400. No printed-CLI change makes this pass; the fix belongs in the generator's matrix fixture synthesis (retro candidate #1).

## Gate
Runner marker: `fail` (5 synthetic-input/generated-command tests, unchanged after hardening). Human-assessed: **ship** — every shipping-scope feature works with real input, hand-authored geo error paths are now hardened, and the 5 failures are proven matrix/generated-command artifacts. Surfaced to the user for the promote decision.
