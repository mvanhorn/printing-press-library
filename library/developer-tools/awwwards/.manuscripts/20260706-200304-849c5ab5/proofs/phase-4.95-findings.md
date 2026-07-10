# Phase 4.95 Local Code Review — Findings Log

**Review path chosen:** direct reviewer-subagent dispatch (correctness, security, maintainability personas in parallel) — the harness /code-review skill is git-diff-shaped and the working tree is not a git repo.

**Autofix summary:** 33 findings autofixed in-place across 2 rounds (security 1, correctness 9 of 10, maintainability 20 of 21, round-2 3). All fixes verified by build+vet+tests and live behavioral checks (find --tech three-js: 0 → 14 matches; trends --limit -1: panic → clean; conflicting filter flags: silent 0 rows → usage error; element traversal input: rejected).

**Not fixed / accepted:**
- Correctness #4 (empty-mirror `[]` sentinel vs object shapes): documented as an explicit sentinel in requireMirror doc + SKILL response-envelope section rather than plumbing per-command zero shapes.
- Maintainability #15 (studio `wins` naming): kept field name; documented that it counts all credited tiers (JSON comment + Long text) rather than breaking the documented shape.

**Convergence outcome:** round 1 = 32 findings → fixed; round 2 = 3 low findings (ALTER error swallow scope, missing limit clamps in find/top, querySet third candidate) → fixed; round 2 explicitly verified all round-1 fixes clean (LIKE ESCAPE, queryStrings nil tolerance, tag candidates, ctx propagation, mirror control flow). Declared converged: round-2 remnants were three one-liners of shapes already vetted elsewhere in the same review.

**Template-shape retro candidates (not fixed in place, route to machine):**
- internal/cli/helpers.go emits writeNoop unconditionally; dead in no-auth CLIs (removed locally; template should gate emission).
- internal/cli/profile.go template comment leaks "HeyGen Beacon" product name into every printed CLI's SKILL prose.
- scorecard live-check sampler: local-store CLIs produce vacuous [] passes without a seeded fixture.

**Out-of-scope findings:** none (reviewers respected internal/cliutil + cobratree exclusion).

**Surface-to-user findings:** none — no fix required a real tradeoff.

**Post-fix simplification:** consolidation happened inside the review loop (queryStrings helper replaced 5 duplicate drain loops, ~90 LoC removed; passthrough helper inlined; filler declarations deleted); a separate /simplify pass was not run to avoid churning freshly-reviewed code.
