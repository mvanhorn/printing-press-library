# Phase 4.8 (SKILL review) + 4.9 (README/SKILL/AGENTS audit) — Findings & Resolutions

## Errors (fixed before Phase 5)
1. Mirror prerequisite undisclosed in SKILL Unique Capabilities + Recipes → added "Prerequisite: run mirror..." to every novel feature's why_it_matters (research.json, dogfood-synced to SKILL/README/which.go) + prepended a mirror priming recipe.
2. SKILL paths section claimed credentials.toml/cookies/auth sidecars/secrets for a no-auth CLI → rewritten to actual contents; credential language removed (incl. "credentials left under former root" doctor note).

## Warnings (all fixed)
3. SKILL Command Reference omitted the 10 hand-built commands → added a "design intelligence (hand-built)" section.
4. Marketing headline ("world's only...finally queryable") → replaced with concrete capability sentence, propagated to all 9 surfaces (root.go Short/Long, SKILL, README, goreleaser, agent_context.go, mcp/tools.go, manifests).
5. README doctor --dry-run comment overstated ("verifies reachable") → reworded (source-fixed in research.json quickstart).
6. state dir description ("persisted queries, jobs, teach.log") → generic "runtime state".
7. Response-envelope claim overbroad → scoped to generated endpoint commands; analytics bare-JSON shapes documented.
8. "HeyGen's 'Beacon' pattern" leak in SKILL profiles section → removed (RETRO CANDIDATE: framework template comment leaks product name from unrelated CLI via internal/cli/profile.go).
9. AGENTS.md mutate-state caution implied writes → read-only note added.
10. README "Read-only by default" → "Read-only".
11. SKILL exit-code table missing 4 (permission denied/blocked) → row added.

## Verified-correct highlights (4.9)
Unique Features/Capabilities exactly match novel_features (5/5); font axis correctly absent everywhere; all documented flags/examples parse; exit codes verified; discovery signals match traffic-analysis.json; anti-triggers present; brand casing correct.

## Retro candidates
- scorecard live-check sampler: pre-seed a mirrored fixture for local-store CLIs (vacuous [] passes)
- writeNoop emitted unconditionally by template (dead in no-auth CLIs)
- profile.go template comment references "HeyGen Beacon" (per-product copy leaking into every printed CLI)
