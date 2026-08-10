# Phase 4.95 — Local Code Review

Reviewer: direct subagent dispatch (correctness/security/maintainability
reviewer), scoped to internal/cli/ + internal/store/qsys_migrations.go +
internal/qsys/. Out of scope (generator-reserved): internal/cliutil/,
internal/mcp/cobratree/.

## Autofix summary
8 findings autofixed in-place (1 round):
- compat_check.go + bom_verify.go: findProduct error was swallowed
  (`err == nil && found`); now returns the error like compat_deprecated.go.
- page_get.go: `--version` value now validated against a dotted-version token
  before URL concatenation (no '/', '?', '#', '..' injection).
- integrations.go: LIKE metacharacters (%, _) in the model are escaped with an
  ESCAPE '\' clause.
- coverage.go + connect.go: countRows now returns (int, error); callers surface
  the error instead of silently reporting zero coverage.
- sql.go: documented the read-only-guard invariant (modernc executes only the
  first statement; guard must not be relaxed or the driver swapped).

## Review verdict
No exploitable SQL/FTS5 injection, no path traversal, no NULL-scan or
drain-order defects. Rate limiting present (AdaptiveLimiter 2 req/s with typed
RateLimitError). Harvest URL construction filters to vendor hosts. All findings
cleared in round 1.
