# Phase 4.95 local code review — youtube reprint 20260819-035139

## Review path chosen
Direct reviewer-subagent dispatch (Agent tool): correctness + security + maintainability lenses in parallel over the hand-authored/hand-modified in-scope files. Round 1 ran before the operator's no-Fable-subagents ruling landed (inherited model); round 2 verification runs on Sonnet per that ruling, consolidated into 2 agents covering all 24 round-1 findings (correctness+maintainability fix-verify merged into one prompt; security bypass-hunt separate).

## Autofix summary
24 findings autofixed in-place across 1 fix round (security: 1 error + 3 warnings; correctness: 1 error + 8 warnings; maintainability: 15 warnings, several overlapping). Key fixes: workspace path-traversal guard on --delete-data + name validation; thumbnail host allowlist (https + ytimg/ggpht incl. redirects) + video-id validation before filesystem use; --key-stdin intake; single-connection BEGIN IMMEDIATE for analyst schema DDL; comment writes switched to UPSERT with AFTER UPDATE FTS trigger (fixes silent FTS corruption both reviewers converged on); snippet.channelId parsed with non-empty-preserving CASE; shared parseChannelListItems + single comment-insert path (kills two guaranteed-drift duplications); watch-note preservation; COLLATE NOCASE handle matching; channel-scoped skipped-count; partial-fetch disclosure in breakouts/packaging; rune-safe truncation; sort.Slice ranking; quota-formula clarity; knownIDOverfetch named constant; env-swap invariant comment.

## Rejected finding (with reason)
- JSON envelope casing unification to camelCase: REJECTED. `fetch_failures` and `scanned_<unit>` are press-mandated envelope keys (skill Phase 3 checklist #12/#13); the metric keys (views_per_day etc.) stay in the same style for family consistency and are already referenced by shipped recipes (`--select items.views_per_day`).

## Template-shape retro candidates
- printJSONFiltered HTML-escapes JSON output (framework helper; surfaced as < in hints) — worked around by rewording, not patched (generator-owned).
- Generator emits dead helper `successfulNoop` (removed locally; machine bug).
- Framework help texts (teach/sync/learnings/profile) alone blow the MCP token-efficiency budget (trimmed locally; machine bug).
- Scorecard live sampler intermittent SIGBUS (roaming commands, never reproducible outside sampler incl. 9-process concurrency tests) — suspected binary staging/refresh race.

## Out-of-scope retro candidates
None (no findings in internal/cliutil/ or internal/mcp/cobratree/).

## Surface-to-user findings
None — all fixes were mechanical per the autofix policy; the one judgment call (envelope casing) is documented above as a rejection, not a user tradeoff.

## Convergence outcome
Round 1 findings all addressed; round 2 (Sonnet) verification in flight at time of writing — outcome appended below.

## Post-fix simplification
/simplify skipped with reason: the autofix round itself performed the consolidations (shared channel parser, single comment insert path) and introduced no defensive duplication; remaining tree is generated code out of simplify scope.

## Round 2 outcome (appended)
- Security (Sonnet): ROUND 2 PASS — all fixes verified, active bypass attempts (dot-anchored host suffix, base==home separator check, --dir escapes, lookalike domains, stdin flooding) all defeated.
- Correctness (Sonnet): 8/9 verified; caught 3 patch sites where round-1 sed edits had silently missed (breakouts partial-fetch + store-error disclosure, comments-mine questions error swallow). All 3 re-applied with grep receipts (breakouts.go:148,206; comments_mine.go questions now returns apiErr). Build + tests green after re-fix.
- Convergence: findings cleared at round 2.
