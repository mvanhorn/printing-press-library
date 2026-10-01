<!-- slop-gate: off -->
# Phase 4.8 / 4.9 documentation review

Reviewers: two read-only subagents (SKILL semantic review; README/SKILL/AGENTS correctness audit).

Phase 4.8: 2 errors, 8 warnings, 5 minor. Phase 4.9: 7 errors, 19 warnings. All errors and warnings fixed except the items under "Deferred".

Fixed (source of truth where generated):
- Freshness contract listed ~48 nonexistent command paths (generator keyed it by sync resource names); x-cache removed from the spec, section and auto-refresh hook gone.
- Overclaims: "Every Tailscale admin API endpoint" (tailnet deletion omitted), fleet wording, "aren't available in any other tool", contractor-access claim on shares revoke.
- Disclosures added: safe-write commands need --yes in every mode and are plan-only over MCP; `self` needs the local tailscale command; search is local-only; sync's non-critical acl/user-invites errors; doctor's OAuth-only "not configured".
- Quickstart step 1 now `doctor` (the old `doctor --dry-run` comment claimed checks it does not run).
- Env var table: TAILSCALE_TAILNET optional (default '-'); OAuth vars added; MCPB tailnet field optional with default '-'.
- `mock-value` placeholders, `auth-status`, `echo $TAILSCALE_API_KEY`, generic "list command", template sports examples, teach example missing --resource-type, AGENTS.md placeholders, relative /docs links, data/state path rows.
- Exit code 1 documented.
- Command reference regenerated from the binary's agent-context tree (92 endpoint commands; the template only rendered one level).
- policy add-entry description now lists nodeAttrs, grants, acls, ssh; search Short now says local.

Deferred (generator template issues, retro candidates):
- Generated Short text is cut at the first period even inside markdown links (e.g. `device name set`).
- Framework help examples use non-Tailscale placeholders (analytics, tail, which, teach, `profile save --region US`).
- Install lines use the mvanhorn module path, like every other RonanRx CLI; `go install` needs the public repo.
- Generator boilerplate in SKILL.md uses em dashes.

verify-skill after fixes: all checks passed.
