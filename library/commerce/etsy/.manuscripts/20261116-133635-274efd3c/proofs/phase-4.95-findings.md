# Phase 4.95 — local code review (tiered peer-review gate) — etsy-pp-cli

**Status: CLOSED 2026-11-17.** The final round-9 confirmation (code + test, narrowed by user decision) returned SHIP from both lenses. No Blocker or Major is open.

- **Working copy:** $CLI_WORK_DIR, main @ e4785c2 (local git, no remote).
- **Rounds:** 1 (8 lenses), 2–3 (6), 5 (6), 7 (5), 8 (3), 9 (2). Fix batches B/C/D/A, E/F, G/H, I/J, K/L, M/N. All builds ran in dedicated worktrees, and every fix was mutation-proven.
- **Gate at final HEAD:**
  - hermetic `go test -count=1 ./...`: 15/15 packages ok;
  - shipcheck run 11: 7/7 legs PASS, 159/159 verify, OAuth 76/76, novel 15/15, scorecard 96/100 (Grade A).
- **Dispositions, tier rationale, deferrals and claim-vs-reality corrections:** $API_RUN_DIR/review/dispositions.md.
- **Durable log:** workshop docs/agent-reviews/2026-11-16-etsy-pp-cli-peer-reviews.md, plus the -raw/ bundle.
- **Behaviour changes a user would notice:**
  - write commands refuse without --yes; --dry-run previews;
  - one exit policy: quota, --max-calls and reserve stops exit 7; failures exit non-7, and a 403-only payments run with no progress exits 1;
  - buyer data is hidden over MCP unless ETSY_MCP_ALLOW_BUYER_PII=1 is set. This covers sql, search, recall, learnings, playbooks, orders --full, analytics on buyer types and reviews digest --text, and is enforced by the internal/mcpconformance gate;
  - payments progress is saved per receipt.
