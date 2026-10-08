<!-- slop-gate: off -->
# Phase 4.95 local code review findings

Review path chosen: direct subagent dispatch via the Agent tool, three personas in parallel (correctness, security, safety+maintainability), read-only, scoped to hand-written files (internal/cli novel commands, internal/dropbox, internal/store/dropbox_migrations.go). Fixes delegated to Codex (CODEX_MODE) and verified by the orchestrator.

## Round 1
- 38 raw findings (correctness 12, security 11, safety/maintainability 15) merged to ~32 unique. Highest-severity items were reported independently by 2-3 reviewers: undo turning a rev-less deleted file into a folder; non-ASCII MoveDropboxPathPrefix corruption (SQLite substr counts characters, Go len counts bytes); journal written after the fact so accepted-then-timed-out batches were marked failed and became un-undoable.
- Autofix: slice I1 (commit bce730d) for apply/undo/plan check/journal integrity; slice I2 for index, analysis, and surface fixes.

## Template-shape retro candidates (not patched in place)
- Generated AGENTS.md documents `--data-source auto|local|live` for novel commands, but the flag is not registered on them (`overview --data-source local` -> unknown flag).
- Generated README/SKILL say `config.toml` while the generated config resolver reads `config.json`; README renders an empty platform-default config path literal ("The platform-default config path is ``").
- Generated SKILL/README include off-domain examples (sports teams, seasons, a `--since` feedback example) and cookie/browser-session boilerplate in an OAuth-only CLI.
- Generated SKILL/README templates use em dashes as list separators throughout.
- Generated `doctor --dry-run` prints a stub but is recommended as quickstart step 1 with "check config and auth wiring" semantics.
- Generated `auth setup` prints "No setup URL is configured" unless the spec sets auth.key_url (spec fixed here).

## Surface-to-user findings
- None requiring a tradeoff decision; all fixes preserve approved scope.

## Round 2
- All 15 round-1 items verified FIXED by all three reviewers (some with follow-up gaps).
- ~25 new findings merged into slice J: CheckPlan O(ops x index) slowdown (measured 23s for 1000 moves / 300k entries), intra-batch dependency gap, nested-delete false errors, public/no-expiry revokes blocked by the dangling guard, unknown_link only warning, attested nonempty deletes invisible in preview, --no-refresh on incomplete roots, preview ignoring allow flags, abort printing status complete, undo --force overloading retry with overwrite, benign undo skips counted as failures, pending ops never reconciled, case-only renames unsupported by move_batch_v2, conflicts plans without keeper guard, legacy index bind dead end, upload guard bypasses, MCP annotation fixes for links audit and files download.

## Template-shape retro candidate (high)
- internal/generator/templates/mcp_tools.go.tmpl emits WithDestructiveHintAnnotation(false) for every non-read POST endpoint. RPC-style APIs (all-POST) therefore advertise delete/move/revoke/delete-all-closed tools as non-destructive, so MCP hosts may auto-approve them. Contradicts AGENTS.md "wrong annotations are worse than missing ones". Printed CLI patched in internal/mcp/tools.go (recorded in .printing-press-patches); machine fix belongs in the template + goldens.

## Round 3
- Round-2 items A-P verified FIXED (P partial, style). Not converged: 11 new findings (2 high: retried undo skipping unrestored children of an unknown folder delete; apply deleting after dependent moves failed).
- Surfaced to user per the 3-round cap. User decision (2026-10-07): fix all remaining findings in one more Codex slice (K), then run a targeted re-review of only that diff before publishing. User also approved a separate generator fix PR for the POST destructiveHint bug.

## Targeted re-review of slice K, then slice L
- Targeted reviewer confirmed K items 1-9 fixed with tests that fail on old code; found 4 regressions (range predicate ignored by planner, skipped-parent child delete, keeper loop O(n^2), unpaired attestation) + MCP rebind gap.
- Slice L (commit 4ae1621) fixed all five. Orchestrator verified on a copy of the real 2.4M-entry index: EXPLAIN QUERY PLAN shows SEARCH on dbx_files_file_path_size for both queries (no SCAN); 40 top-level folder count queries 162 ms total. Named tests pass for skipped-parent, rebind TTY gate, account mismatch, keeper scale, paired attestation.
- Convergence outcome: findings cleared after round 3 + targeted re-review + verification (user-approved extension past the 3-round cap).

Autofix summary: findings fixed in place across 3 review rounds plus a user-approved targeted re-review; see commits bce730d (I1), 9c33ac9 (I2), 982908b (J), a4e14aa (K), 4ae1621 (L), plus f27d661 (MCP destructive hints, docs) and the slice M commit.
Convergence outcome: cleared after round 3 + targeted re-review + verification.
