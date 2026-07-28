# Phase 4.95 — Local Code Review Findings

## Review path chosen
Direct reviewer-subagent dispatch via Agent tool: ce-correctness-reviewer, ce-security-reviewer, ce-maintainability-reviewer (always-on set), scoped to the 8 hand-written novel files + tests under internal/cli/.

## Autofix summary
16 findings autofixed in-place across round 1 (correctness: 1 high + 3 medium + 6 low; security: SEC-1 P2 + SEC-2 P3; maintainability: MAINT-1..4). Key fixes: guard poll loop now treats unknown statuses as terminal with a hard poll cap and its own --wait deadline (15m default) decoupled from the per-request --timeout; audit usage records fetch_failures on unparseable/empty limits instead of reporting a false "ok", validates account UUIDs before path splicing, and caps pagination at 100 pages; guard re-validates mirror-resolved install ids against uuidRe and distinguishes DB errors from not-found; wildcard cert comment corrected to RFC 6125 semantics (apex NOT covered); DaysLeft uses math.Floor; timestamps normalized to UTC before sort; env-filtered summary count; unknown_version_installs surfaced; drainRows helper unifies 9 drain-first sites; gofmt clean. Round-2 re-review dispatched for convergence.

## Template-shape retro candidates (NOT fixed in place — machine bugs)
1. **`auth set-token` referenced by generated code for a Basic-auth CLI whose real subcommand is `auth set-credentials`.** Sites: internal/cli/auth.go:138 (Example), internal/cli/doctor.go:478,480 (warnings), internal/client/client.go:861 (credential error). Also README agentcookie section said `auth-status` (hyphen; hand-fixed in README only). Template should derive the auth-subcommand name from auth type.
2. **regen-merge inconsistency:** on `generate --force`, preserved run-1 `internal/mcp/tools.go` (calls RegisterIntents) while fresh tree no longer emits `intents.go` (recipe-derived intents changed) → unbuildable merge. Repaired by adopting fresh tools.go/tools_test.go. regen-merge should invalidate a preserved templated file when a conditionally-emitted sibling it references disappears from the fresh tree.
3. **README MCP install section emits only the first auth env var for HTTP Basic pairs** (WP_ENGINE_API_USERNAME without WP_ENGINE_API_PASSWORD in the Claude Desktop JSON and the "Fill in" step). Hand-fixed in README; template should render every required env var.
4. **README config-path paragraph uses a display-name-derived slug** (`wp-engine-hosting-pp-cli`) instead of the real appName (`wpengine-pp-cli`). Hand-fixed; template should use the canonical appName.
5. **SKILL `which` boilerplate claims exit 2 on no-match without the --json/--agent exit-0 caveat.** Hand-fixed; template text should carry the caveat.

## Out-of-scope retro candidates
None (no findings in internal/cliutil/ or internal/mcp/cobratree/).

## Surface-to-user findings
None — no real-tradeoff findings; all fixes were mechanical.

## Residual risks (accepted, documented by reviewers)
- audit_usage linear projection is rough at month boundaries (daysElapsed=1 sliver); model limitation, documented in help.
- whois picks the first covering cert when multiple cover the domain; SQLite ordering unspecified.
- guard purge-failure path prints view JSON + apiErr (exit 5) — intentional, documented in Long text.

## Convergence outcome
Round 1 complete; round 2 dispatched (see below for outcome).

## Round 2 outcome
- security: CONVERGED — no findings (SEC-1/SEC-2 fixes verified).
- maintainability: CONVERGED — no findings (MAINT-1..4 verified; drainRows at 9 sites).
- correctness: 3 new findings, all fixed in round 3:
  1. versionBelow("8.2","8.2.0") returned true (empty segment compared as string) — normalized missing segments to "0"; test cases added.
  2. guard "pending" (poll-cap) exited 0, contradicting the documented CI contract — now exits non-zero (apiErr) outside dogfood env.
  3. whois cert_expires formatted without .UTC(), inconsistent with audit certs — fixed.
  Plus residual-risk closure: audit usage now records a fetch_failure when the summary body has none of the expected metric keys (silent-zero projection guard).

## Template-shape retro candidates (addendum)
6. **Generated `internal/cliutil/credentials_test.go` fails out of the box** on this Basic-auth CLI: TestEmptyCredentialsFileDoesNotClearLegacyConfig — `AuthHeader() = "", want legacy credential`. Reproduced identically in an untouched fresh generation (scratchpad/wpe-fresh), so it is a generator template bug, not a hand-edit regression.

## Round 3 outcome (final)
- correctness: CONVERGED — no findings; all four round-2 fixes verified (versionBelow normalization + tests, guard pending exit contract, whois UTC, usage summary shape guard).
- Round-3 scope judgment: only correctness re-dispatched — the four diffs were one-liners inside correctness's own findings; security and maintainability had already converged on the surrounding code and the diffs introduced no new surface in their domains.

## Convergence outcome (final)
Findings cleared at round 3.
