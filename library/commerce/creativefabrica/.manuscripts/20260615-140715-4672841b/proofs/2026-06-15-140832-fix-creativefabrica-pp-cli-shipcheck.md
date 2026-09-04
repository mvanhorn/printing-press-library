# Creative Fabrica CLI — Shipcheck + Polish Summary

## Shipcheck (umbrella): PASS (6/6 legs)
verify PASS (26/26, 100%) | validate-narrative PASS | dogfood PASS |
workflow-verify PASS | verify-skill PASS (0 findings) | scorecard PASS (leg exit 0)

## Phase 5 Full Dogfood: PASS — 49/49 live checks
## Phase 4.95 local code review: 2 mechanical fixes (filter-injection escaping, Retry-After). Credential handling PASS.
## Phase 5.5 Polish: HTML-entity slice leak fixed; 3 hand-authored gosec findings cleared (0 remaining hand-authored).

## Post-polish hand fixes (this agent)
- Fixed broken `--select name_en,regularPrice` recipe (field names are name/regular_price) in README + SKILL + research.json.
- Removed inaccurate "local SQLite database" claim from README/SKILL/research narrative (CLI uses file-based snapshots, not a SQLite mirror).
- Added README "Why this exists" (vision) + "Workflows" sections + full command documentation (was: only the low-level `products` passthrough).

## Scorecard: 64/100 (Grade C) — held at the 65 floor by STRUCTURAL dimensions
- vision 0/10, workflows 2/10: not README-movable (verified by re-score); these measure
  sync/multi-entity-workflow patterns a read-only single-source search CLI cannot satisfy.
- data_pipeline_integrity 1/10, sync_correctness 5/10: no bulk sync (20M-item catalog can't be mirrored).
- dead_code 1/5 + 1 dead flag: all in generator-emitted DO-NOT-EDIT files (unconditional pagination/partial-failure scaffolding). Retro candidate.
- MCP "1 tool": scorecard counts static spec endpoints; the runtime MCP correctly exposes 15 tools. Retro candidate.

## Verdict: hold (scorecard 64 < 65), per polish recommendation.
The CLI is functionally complete and fully tested; the cap is structural + generator-owned.
ship_recommendation: hold | further_polish_recommended: no | recommended next: retro (generator signals).

## Known Gaps (documented)
- Authenticated personal library/favorites/downloads (cookie-auth GraphQL) — deferred to /printing-press-amend.
- Zero-setup credential auto-discovery is best-effort (CF blocks plain HTTP on homepage); env var / `auth set-key` is the reliable path. Full uTLS auto-discovery = future enhancement.
