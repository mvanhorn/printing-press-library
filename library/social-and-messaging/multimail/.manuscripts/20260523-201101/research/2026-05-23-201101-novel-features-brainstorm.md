# Novel Features Brainstorm — MultiMail CLI

## Customer model

### Persona 1: Mira — CI/CD Pipeline Engineer
**Today:** Maintains GitHub Actions workflows orchestrating AI agents (Codex, Claude Code) for automated code review/triage. Wires raw curl calls to MultiMail API, manually parses JSON, stores API keys as GitHub secrets. No local view of agent sends/receives.
**Weekly ritual:** Monday: review oversight approval queue across three agent mailboxes, approve/reject pending gated sends, check for weekend-stuck approvals, scan for allowlist gaps, review usage quotas.
**Frustration:** Oversight queue is per-mailbox with no cross-mailbox view of pending approvals, approval velocity, or bottlenecked mailboxes. Clicks through dashboard mailbox by mailbox or writes throwaway scripts.

### Persona 2: Dawit — Agent Developer (Solo/Startup)
**Today:** Building AI customer-support agent handling inbound email. Uses MCP tools in Claude, falls back to curl in terminal/SSH/Docker. Frequently needs to search old emails to understand threads, but API only supports per-mailbox listing.
**Weekly ritual:** Tests new agent behavior: send test emails → watch arrival → check draft replies → approve via oversight → progressively upgrade mailboxes from gated_all → gated_send → monitored. Checks audit log for agent behavior vs expectations.
**Frustration:** No single view of trust ladder progression across mailbox fleet. Six mailboxes at different oversight levels with no way to see which are at which level, last upgrade date, or readiness for next step. Can't search email content across mailboxes.

### Persona 3: Keiko — Compliance/Ops Lead
**Today:** Manages 15 AI agents across enterprise MultiMail deployment. Ensures communication policy compliance: no unapproved domain emails, oversight decisions within SLA, complete audit trails. Uses dashboard + spreadsheet exports.
**Weekly ritual:** Pull audit logs, filter oversight decisions, calculate approval/rejection rates per mailbox, verify no emails sent outside allowlist, produce agent communication activity summary. Monitor suppression for bounces/spam indicating misbehaving agent.
**Frustration:** Weekly compliance snapshot is manual multi-step: pull audit events + oversight decisions + email send counts + suppression entries, correlate in spreadsheet. No "agent communication health" view. Allowlist-vs-actual-sends gap analysis especially painful.

## Candidates (pre-cut)

12 candidates generated. Inline kill/keep applied.

## Survivors and kills

### Survivors (6/12, all >= 5/10)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Oversight velocity | `oversight velocity` | 8/10 | Joins audit_events × oversight decisions in local SQLite; computes approval/rejection rates + median decision latency per mailbox cross-fleet |
| 2 | Trust ladder status | `trust status` | 7/10 | Joins mailboxes × audit_events for upgrade/downgrade actions; shows current mode, time-at-level, upgrade history per mailbox |
| 3 | Allowlist coverage | `allowlist coverage` | 7/10 | Joins allowlist_entries × sent emails; matches exact + wildcard patterns against recipients; reports coverage percentage per mailbox |
| 4 | Inbox health | `inbox health` | 7/10 | Aggregates synced emails: unread count, oldest unread age, reply rate, avg thread depth, per-mailbox breakdown |
| 5 | Stale threads | `threads stale` | 6/10 | Queries threads JOIN emails WHERE MAX(received_at) < threshold; surfaces dropped conversations as agent failure signals |
| 6 | Cross-mailbox search | `search` | 8/10 | FTS5 across all synced mailboxes in local SQLite; API search is per-mailbox only |

### Killed candidates (6/12)

| Feature | Kill reason | Closest survivor |
|---------|-------------|-----------------|
| Compliance snapshot | Scope creep — 4 data sources achievable by piping survivors #1 + #3 | Oversight velocity + Allowlist coverage |
| Quota forecast | Requires historical usage snapshots sync may not accumulate; single usage call + math covers 80% | Inbox health |
| Oversight pending summary | Subsumed by oversight velocity which includes pending counts + temporal analysis | Oversight velocity |
| Audit digest | Generic filter/group achievable with audit-log list --json | jq | Oversight velocity |
| Mailbox diff | Niche — rare operation; trust status covers most important dimension | Trust ladder status |
| Sync status | Plumbing, not user value — belongs as --verbose on sync | Cross-mailbox search |
