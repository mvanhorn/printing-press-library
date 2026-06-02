# Zapmail Novel Features Brainstorm (audit trail)

## Customer model

**Priya - Cold Email Agency Operator (owns deliverability for 40+ clients)**
Runs a DFY cold email agency; ~25 isolated workspaces, ~150 domains, ~600 mailboxes. Dashboard shows one workspace at a time. Monday fleet sweep takes 90 min of clicking and still misses unhealthy domains / failed mailboxes / warmed-but-unassigned inboxes. No cross-workspace view exists.

**Marcus - Outbound Lead at a Series-B SaaS (runs his own infra)**
One workspace, ~12 domains, ~80 mailboxes feeding SDR Smartlead. Cost-conscious, reports to a VP. Friday cost-and-capacity check: paid vs active mailboxes, what renews next week. Billed for purchased mailboxes whether assigned or not; no cost-per-active-mailbox number; renewal dates buried per-domain; missed renewal silently drops a domain.

**Devin - AI-SDR Platform Builder (embeds Zapmail mgmt in a product)**
Drives Zapmail from scripts/Claude Code. Nightly sync to SQLite, then queries for stalled exports + failed mailboxes. Wants --json/--select/--csv + typed exit codes. Neither the community MCP (one action at a time) nor the dashboard gives a queryable local mirror or a watch loop.

## Survivors (transcendence features)

| # | Feature | Command | Score | Buildability | Persona |
|---|---------|---------|-------|--------------|---------|
| 1 | Fleet health rollup | `analytics --type fleet-health --group-by workspace` | 8/10 | hand-code | Priya |
| 2 | Warmed-but-unassigned finder | `mailboxes idle` | 8/10 | hand-code | Priya, Marcus |
| 3 | Failed-mailbox triage | `mailboxes failed` | 7/10 | hand-code | Priya, Devin |
| 4 | Renewal cost forecast | `analytics --type renewals --group-by week` | 8/10 | hand-code | Marcus |
| 5 | Cost-per-active-mailbox | `analytics --type cost-efficiency --group-by workspace` | 7/10 | hand-code | Marcus |
| 6 | Stalled-export finder | `exports stalled` | 7/10 | hand-code | Priya, Devin |
| 7 | Capacity gap report | `analytics --type capacity --group-by workspace` | 7/10 | hand-code | Marcus, Priya |

## Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Export-status watch (`exports watch`) | Thin wrapper on /v2/exports/status; folded into `exports status --watch` flag | `exports stalled` (#6) |
| Cross-workspace fleet search | Already absorbed as framework `search` over FTS index | Framework `search` |
| Domain DNS-completeness audit | dns_records fetched per-domain on demand; mirror not guaranteed complete | `analytics --type fleet-health` (#1) |
| Fleet drift / reconcile | Overlaps/reimplements framework `sync` | Framework `sync` + `mailboxes failed` |
| Pre-export readiness gate | `exports mailboxes --dry-run` already prints the batch | `exports mailboxes --dry-run` + `mailboxes idle` |
| Abused-domain alert list | `is_abused` already surfaced by fleet-health rollup | `analytics --type fleet-health` (#1) |
| Wallet runway forecast | Derivative of renewals minus one balance number | `analytics --type renewals` (#4) |

Note: the 4 `analytics --type` variants were tagged `spec-emits` by the subagent but require hand-written aggregation logic over the local store; treating all 7 as `hand-code` for Phase 3 scope.
