# Polish — wanderlust-goat v2 reprint

| Metric          | Before    | After     | Delta |
|-----------------|-----------|-----------|-------|
| Scorecard       | 85/100    | 85/100    | 0     |
| Verify pass-rate| 100%      | 100%      | 0     |
| Verify-skill    | 3 errors  | 0 errors  | -3    |
| Tools-audit     | 0 pending | 0 pending | 0     |
| go vet          | 0         | 0         | 0     |

**Verdict:** ship

## Fixes applied
- Fixed SKILL/README example for `research-plan`: replaced invalid `--identity` flag with `--anchor`/`--country`, restructured the example so criteria is positional and anchor is a flag (matches the command's actual interface).
- Fixed SKILL/README example for `sync-city`: replaced invalid `--layers all` flag with `--country JP` (sync-city fans out to all implemented Stage-2 sources for the country automatically; there is no per-layer toggle).
- Updated quickstart and recipes in research.json to use the corrected flag forms; re-ran dogfood to propagate the changes through README/SKILL.

## Skipped (known machine-side gaps)
- `extractResponseData` dead helper: emitted by generator template into a "DO NOT EDIT" file, used only by promoted-command template, unused in this novel-feature-only CLI. Systemic / retro candidate.
- 13 novel-feature client files lack rate-limit handling (atlasobscura, googleplaces, osrm, overpass, tabelog, naverblog, navermap, lefooding, hotpepper, reddit, retty, wikipedia, dispatch): hand-written HTML/API clients; adding rate limiters is feature-add scope. Generator-level guidance for novel-feature client templates would be a retro candidate.
- `mcp_token_efficiency` 4/10 and `mcp_remote_transport` 5/10: spec-edit fixes that require regen — out of scope for mid-pipeline polish.
- `mcp_description_quality`, `mcp_tool_design`, `mcp_surface_strategy` unscored: only 2 typed endpoint tools, structural.
- `cache_freshness` 5/10: scorer flags "cache freshness helper not emitted" — generator concern.
- Phase 4.85 output-review SKIPPED: no research.json adjacent to CLI binary in `.runstate/` working dir.
