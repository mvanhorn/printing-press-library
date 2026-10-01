<!-- slop-gate: off -->
# Phase 4.95 local code review

Review path: Codex CLI (`codex exec --sandbox read-only -m gpt-6-astra -c model_reasoning_effort=high`), two rounds, scoped to the hand-written files under internal/cli and internal/tsadmin.

Autofix summary: 8 round-1 findings (5 P1, 3 P2) fixed in place; round 2 confirmed 7/8 and raised 2 P2s, 1 fixed (symlinked backup files rejected) and 1 not applicable (legacy backup directory: first release, no backups exist in the old layout). Regression tests added for each fix plus a mutation matrix (agent mode without --yes, --yes --dry-run) across the hand-written write commands.

Round 1 fixed: shares revoke filters require --device; failed revocations exit 6 and discovery failure deletes nothing; OAuth token cache keyed by endpoint + client id + secret digest; access check sends HuJSON and reports indeterminate when a matched line holds more than one rule; route result mismatch exits 5; share fan-out keyed by nodeId; expiry key-scan failure exits non-zero and warnings render; backup directories are slug + hash, sidecar IDs validated, containment checked.

Convergence: findings cleared at round 2 (remaining item is not applicable to a first release).

Template-shape retro candidates (not fixed in place; generator issues):
- Generator promotes a single-endpoint DELETE onto the resource parent (`tailnet` would delete the tailnet). Worked around by dropping the operation from the spec.
- Search-endpoint heuristic matches any path containing "search", including POST /dns/searchpaths (a settings write). Worked around with a root hook forcing local search.
- Read-only inference from operationId `get*` on a POST (create-or-get AWS external ID); doctor suggests it as a credential check.
- Env-backed path template vars produce NOT NULL scope columns that sync never fills.
- Cache freshness registry keyed by sync resource names, not command paths, for nested resources.
- SKILL/README command reference renders only one level of commands.
- Generated Short text cut at the first period inside markdown links.
- MCP `tailscale_execute` runs mutating endpoints without a confirm/dry-run gate (upstream work in progress).

## Post-fix simplification (/simplify, 4 parallel cleanup reviewers)
Applied: raw policy requests and the OAuth exchange now go through the generated client's HTTP client (one config load, its redirect guard, shared transport); auth errors prefer the generated credential-refusal message; path segments use cliutil.EscapePathParam; one generic getLive helper replaces six fetch/decode copies; add-entry and restore share finishPolicyChange; emitOrRender replaces three emit/render/held-error blocks; shareRowOf builds share rows in audit, revoke, and inspect; devices inspect overlaps the slow audit-log call with the other reads and runs `tailscale status` at most once (with --peers=false); AddEntry parses once and uses deep Clones; RuleStartLines counts lines forward; Retry-After via cliutil.RetryAfter; newTabWriter and partialFailureErr reused; dead code removed (DaysLeft, SortedCopy, JoinHostPort, tsOAuthConfigured, PeerIDs, revoke Warnings); exit-node partial warning now counts only exit routes; the safe-write commands no longer carry pp:mutation (plan-only without --yes already), so the --list special case is gone; sync stamps the tailnet the request actually used.
Skipped (value too low or behavior tradeoff): parallelizing access check and expiry reads, fan-out for revoke-by-ID, reading routes from the fields=all device list, honoring a config-file tailnet in --tailnet flag defaults.
