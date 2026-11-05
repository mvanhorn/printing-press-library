# Phase 4.95: Local Code Review Findings

- **Autofix summary:** 2 findings autofixed in-place across 1 round (`ExtractMatchups`/`ExtractDraftPicks` slice deduplication in `internal/yahoofantasy/yahoo.go` and `waiver-wire` recipe `--select` field names in `research.json`).
- **Template-shape retro candidates:**
  - `internal/cli/root.go`: 4 unused helper functions (`hasChangedLocalFlags`, `requiredInputWaived`, `retainCLIQueryParams`, `retainExplicitQueryParams`) emitted by generator template when no endpoint uses those specific flag-waiver paths.
- **Out-of-scope retro candidates:** None.
- **Surface-to-user findings:** None.
- **Convergence outcome:** Findings cleared at round 1.
- **Review path chosen:** Direct code review audit across `internal/yahoofantasy/`, `internal/cli/`, `README.md`, `SKILL.md`, and `AGENTS.md` (verified `boundCtx(cmd.Context(), flags)`, `sql.NullString` scan targets, parameterized SQL queries, and OAuth token refresh response bounds).
