# Phase 4.95 Local Code Review — Findings

## Autofix summary
8 of 9 findings autofixed in-place across 1 round (no git repo in runstate; diffs are in the working tree). Fixed: (1) sortCampaignRefs — "most recent N" cap now operates on a recency-sorted list in both local and live paths; (2) engagement + bounce-audit aggregation restricted to in-window campaigns via inWindowCampaignKeys/sqlInPlaceholders, with an honest note when window membership is unknown; (3) bounce-audit last_campaign now resolved by real sent time, not MAX(campaign_key); (4) delta Metrics initialized to []; (5) missing-mirror early exits emit the typed empty view via printJSONFiltered instead of a bare []; (6) machine-output condition extended with flags.csv/quiet/plain in all six novel commands; (7) digest local list-query errors surface in fetch_failures / return instead of being swallowed; (8) snapshot dedup now compares spams + hard/soft bounces. Also dropped the redundant idx_crs_key_time index (part of #9).

## Declined findings
- #9 (maintainability, LOW): drainRows/live-preamble consolidation refactor declined this round — the duplicated pattern is correct and test-covered; consolidation is churn without behavior change. Candidate for a later polish pass.

## Template-shape retro candidates
- (from Phase 3, restated) internal/cliutil AtomicWritePrivateFile: Chmod-only privacy is a no-op on Windows DACLs; its own VerifyCredsPerms rejects the file it writes. Fix shipped in this CLI (RestrictPrivateFile platform pair + pre-rename hook); generator should absorb it.
- Generated tests set HOME but not USERPROFILE; on Windows os.UserHomeDir() reads USERPROFILE — every suite touching config/state leaks into the real home dir.
- Generated newAuthSetTokenCmd emitted but unregistered for oauth2_refresh specs.
- Sync engine does not apply spec param defaults (resfmt=JSON) — silent 0-record syncs; worked around via query-embedded spec paths.
- Syncer ID extraction misses `listkey` — mailing lists uncacheable offline; spec-level id_field hint needed.

## Out-of-scope retro candidates
- None beyond the template-shape items above.

## Surface-to-user findings
- None — no finding required a real tradeoff.

## Convergence outcome
Findings cleared at round 1 (8 fixed, 1 declined with reason).

## Review path chosen
Direct subagent dispatch (single combined correctness+security+maintainability reviewer via Agent tool).
