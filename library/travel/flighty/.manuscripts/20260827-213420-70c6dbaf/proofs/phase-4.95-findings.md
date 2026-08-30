# Phase 4.85/4.95 Review Findings — flighty-pp-cli

## Review paths chosen
- Phase 4.8 + 4.9: direct subagent dispatch (SKILL/README/AGENTS correctness vs binary).
- Phase 4.85: direct subagent dispatch (live output plausibility sampling).
- Phase 4.95: direct subagent dispatch (security + correctness reviewer on hand-authored files).

## Results
- Phase 4.8/4.9: PASS. 2 warnings (SKILL decision-tree key casing uses Go-style names vs snake_case JSON — template boilerplate; learn-loop examples contain by-design agent-substitution placeholders). No errors. Not fixed (generator template text; retro candidates).
- Phase 4.85: STATUS: PASS. All 8 live output-sampling checks honest (filters, sort order, negative cases, no fabrication). Cosmetic note: `region: "All"` appears for airports only present in the All region — world-view artifact, not a bug.
- Phase 4.95: 10 findings (4 medium, 6 low) — ALL autofixed in one round:

## Autofix summary (one line)
10 findings autofixed in-place across 1 round in hand-authored files (snapshot ordering by MAX(id), live-catalog parse errors propagated instead of silent empties, airline fan-out slug resolution hoisted out of goroutines + 4-worker semaphore + deterministic side order, route zero-value sides omitted, --db wired through worst/nearby/airline, unused diff param removed, rsc.go match copy before append).

## Template-shape retro candidates (generator-owned, not fixed in place)
- Dead functions in generated helpers.go: collectionItemsForOutput, hasChangedLocalFlags, successfulNoop.
- Thin Shorts on framework commands: `profile list` ("List client profiles"), `teach list` ("List recorded learnings").
- Generated endpoint `example` values are not flag-validated at generate time (tv example promised --status that the command lacked).
- Framework `feedback` command emitted without an Example section (dogfood help probe requires one).
- Generic `embedded-json` extractor cannot parse Next.js App Router RSC flight chunks.
- SKILL.md decision-tree key casing (Go-style vs snake_case JSON) in the learn-loop template.

Convergence outcome: findings cleared at round 1.
