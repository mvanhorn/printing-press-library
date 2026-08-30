# Acceptance Report: Redmine

Level: Full Dogfood
Tests: 267/267 passed (0 failed)
Auth: api_key (REDMINE_API_KEY, live against the container's self-hosted dev instance)

## Fixes applied during dogfood (before the passing run)
- **CLI fix** — `internal/cli/issues_blockers.go`: `blockerNode.Blocks` was declared but never assigned (always reported `0`). Fixed by tracking parent→child during the BFS walk; verified live (`issues blockers 3` now reports `"blocks": 3` for the issue that actually blocks it).
- **CLI fix** — `internal/cli/issues_blockers.go`: `fmt.Sscanf(args[0], "%d", &id)` silently accepted trailing garbage (`"3abc"` parsed as `3`). Replaced with `strconv.ParseInt`.
- **CLI fix** — `internal/cli/issues_blockers.go`: a single failed fetch mid-BFS discarded the whole partial blocker chain already discovered. Now accumulates and continues, matching the parallel-fetch partial-failure convention.
- **CLI fix** — `internal/cli/workload.go`, `internal/cli/issues_blockers.go`: `Long` help text referenced non-existent sibling commands (`issues list --assigned-to`, `issues relations list`). Corrected to the real paths (`issues-json get-issues --assigned-to-id`, `issues relations-json get-issue-relations`).
- **CLI fix** — added `pp:happy-args` annotations to `roadmap burndown` (`version=1.0;--project=demo`) and `issues blockers` (`issue_id=3`) so the live matrix exercises real fixture data instead of reporting hollow coverage.
- **Printing Press issue (retro)** — the generated `feedback` parent command's Cobra help has no `Examples:` block (only its `list` subcommand does), failing the mechanical help-check in every printed CLI, not just this one. Patched the generated file directly to unblock shipping (template fix still needed upstream).
- **Printing Press issue (retro)** — verify's auth-env classifier treats optional convenience env vars (`REDMINE_SKIP_TLS_VERIFY`, and a global `{project_id}` template-var fallback) as required-and-missing. Non-blocking (verify's own fix loop deferred both as non-critical), but worth tightening upstream.

## Gate
PASS. All mandatory Full Dogfood tests passed; no broken flagship feature.
