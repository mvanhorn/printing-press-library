# Phase 4.95 local code review

Review path: direct subagent dispatch. Round 1: correctness, security and maintainability reviewers in parallel. Round 2: the same three. Round 3: one combined correctness + security reviewer on the round-2 diff (the round-2 maintainability items were wording and consistency, all applied).

Autofix summary: about 45 findings autofixed in place across 3 rounds (no git history in the working copy; the pre-review tree is kept at backup-pre-review and the full diff at the session scratchpad review-diff.patch). Highlights: batch run file/stdin refused over MCP (arbitrary local file read), jobs get id validated and escaped (path traversal), --fields no longer poisons the response cache, news watch memory scoped per query and edition, serp diff and rank competitors compare pages of one exact key, batch searches deduped by query/engine/country/lang, fixed-width timestamps, credits burn rate based on the ledger's real start, terminal job statuses mapped to exit codes, engine aliases in local filters, MCP spend caps (geo compare 10 countries, batch run 50 searches by default and no negative budget, concurrency 1..16), MCP hints (local-write on rank check and news watch, read-only-unless-flags save on the compare commands).

Convergence: findings cleared at round 3 (last round: 1 medium + 2 low, all fixed; go vet clean, go test ./internal/... green).

Template-shape / out-of-scope retro candidates (not fixed in place):
- Generated search commands expose --all (up to 100 pages, so up to 100 credits per MCP call). Suggest the generator caps or disables --all for MCP children.
- Generated sync defaults to --max-pages 0 and is mirrored to MCP; the MCPChildEnvVar doc says MCP children should be treated like dogfood (max one page) but sync does not do it.
- Generated root help "search: Full-text search across synced data" and sync help pointing at search are wrong for an API without list endpoints.
- generate --force on the same spec deleted hand-authored files and reverted hand edits (see build log).

Surface-to-user findings: none.

Post-fix /simplify: not run. The two maintainability rounds already consolidated the duplicated helpers, and another automated rewrite right before the live dogfood would add churn without a reviewer pass.
