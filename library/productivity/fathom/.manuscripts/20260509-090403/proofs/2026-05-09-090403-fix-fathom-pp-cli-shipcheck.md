# Fathom CLI Shipcheck Report

## Shipcheck Results

| Leg | Result | Exit | Elapsed |
|-----|--------|------|---------|
| dogfood | PASS | 0 | 988ms |
| verify | PASS | 0 | 8.511s |
| workflow-verify | PASS | 0 | 12ms |
| verify-skill | PASS | 0 | 210ms |
| scorecard | PASS | 0 | 102ms |

**Verdict: PASS (5/5 legs)**

## Verify: 100% (27/27 commands)
All absorbed and novel commands pass help, dry-run, and exec checks.

## Scorecard: 87/100 (Grade A)
- Novel features: 9/9 survived
- Output modes: 10/10
- Auth: 10/10
- Agent native: 10/10
- MCP Remote Transport: 5/10 (stdio only, no HTTP transport)
- Cache Freshness: 5/10 (cursor-based, incremental)

## What was built
**Absorbed (14 features):** meetings list/get, recordings transcript/summary, teams, team-members, webhooks create/delete, sync, search, export, import, tail, workflow

**Novel (9 features):**
1. `commitments` — Cross-meeting action item tracker by assignee
2. `topics` — FTS5 keyword frequency + week-over-week trend
3. `brief` — Pre-call participant-keyed history by email/domain
4. `velocity` — Month-by-month cadence tracker per external domain
5. `workload` — Per-team-member weekly meeting hour aggregation
6. `account` — Full domain-keyed relationship history
7. `stale` — Store integrity check for missing transcript/summary/action_items
8. `crm-gaps` — CRM-matched meetings with no action items (sales hygiene)
9. `coverage` — Recurring meeting recording coverage by title pattern

## Ship Recommendation: ship
