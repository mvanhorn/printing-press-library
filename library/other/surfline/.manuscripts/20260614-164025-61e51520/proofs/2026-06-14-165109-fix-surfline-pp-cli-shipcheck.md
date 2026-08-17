# Surfline CLI Shipcheck

## Verdict: ship

## Legs (all PASS)
- verify: PASS
- validate-narrative: PASS (strict, full-examples)
- dogfood: PASS (wiring, dead-flag, novel-features 8/8)
- workflow-verify: PASS
- verify-skill: PASS (flag-names, flag-commands, positional-args, canonical-sections)
- scorecard: 90/100, Grade A

## Scorecard highlights
- 10/10: output_modes, auth, error_handling, terminal_ux, readme, doctor, agent_native,
  mcp_description_quality, mcp_remote_transport, local_cache, workflows, insight, path_validity,
  sync_correctness
- 9/10: breadth, vision, agent_workflow_readiness
- Lower (intentional/limits): cache_freshness 5 (no upstream refresh path), type_fidelity 5
  (untyped object responses), data_pipeline_integrity 7, mcp_token_efficiency 7
- Unscored: mcp_tool_design, mcp_surface_strategy, live_api_verification

## Live dogfood (Phase 5)
- Full matrix: 100/100 PASS (status: pass). No token required.
- All 7 novel features validated against the live Surfline API with correct field mappings.

## Fixes applied during shipcheck/dogfood
- doctor optional-auth (env-var ERROR→INFO).
- sync no-op for no-bulk-listable-resources (was HTTP 500 via /search/site).
- offline search rebound to journaled spots (sync can't populate).
- real spotId/subregionId fixtures across examples (fake UUID → working ids).
- buoy-check coords from wave associated.location; buoy parse for {data:[...latestData]}.
- alert run context-cancel-before-use fix.
- empty-is-valid error paths annotated (journal show, spots find).

## Known gaps
None blocking. regions requires a forecast subregionId (spot.subregion._id from `spots report`),
documented in the flag help and example.
