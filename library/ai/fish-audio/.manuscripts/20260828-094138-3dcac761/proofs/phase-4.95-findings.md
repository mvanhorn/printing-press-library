# Phase 4.95 local code review

Review path chosen: direct subagent dispatch (feature-dev:code-reviewer, correctness + security lens) over hand-authored files only.

## Autofix summary
3 findings routed to the build agent for in-place fixes (round 1): zero-shot render unreachable / competing reference_id (tts_render.go), --skip-if-rendered never writes --out (tts_render.go), duplicate batch lines collapse to one render_log row (tts_batch.go + migrations ON CONFLICT). Plus an ordering nit: render diff arg check before dryRunOK.

## Template-shape retro candidates
- internal/cli/helpers.go:748 `truncate` slices by bytes and splits multi-byte runes; JSON output gets U+FFFD, human tables get mojibake. Recurs in every printed CLI. Fix upstream: rune-aware truncate.
- auth.go Short "Manage authentication for Fishaudio" ignores research.json display_name.

## Out-of-scope retro candidates
none

## Surface-to-user findings
none

## Convergence outcome
Findings cleared at round 1 (all 3 fixed with regression tests; shipcheck re-run green except live_api_verification).
