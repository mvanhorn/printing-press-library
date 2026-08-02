# Novel Features Brainstorm — gmail-pp-cli (subagent output, verbatim)

## Customer model

**Priya, the agent operator.** Runs Claude Code sessions all day; wants her agent to handle email as a tool. Today uses the GongRzhe Gmail MCP server: raw base64url blobs, 20 quota units per messages.get, no local corpus. Weekly ritual: Monday agent-driven triage (summarize unread, label, archive, draft replies). Frustration: every MCP call returns undecoded MIME; no agent-shaped --json --select output over a local corpus; triage is slow, quota-bound, re-fetches the same messages.

**Marcus, the inbox-zero power user.** 90k messages; treats Gmail as a queue to drain. Hand-types operator queries, deletes in 50-message pages; cannot answer "which senders fill my mailbox?" from the UI. Weekly ritual: Saturday purge (large/old/promotional hunt, unsubscribe, archive). Frustration: identifying which lists to kill is manual archaeology — no sender-level volume/size/unread aggregates; unsubscribe links buried per-message.

**Sofia, the founder doing outreach.** Sends 20-40 cold/follow-up emails weekly across timezones. Drafts at 11pm, sends at bad hours or forgets drafts; Gmail schedule-send is UI-only. Weekly ritual: draft batch Sunday night, want Tuesday 9am delivery, chase non-repliers. Frustration: cannot schedule sends from any API tool (API lacks it); "who has not replied after 3 days" requires opening every sent thread.

**Ken, the data hoarder analyst.** 1M messages back to 2004; wants mailbox as queryable dataset. Runs GYB for backup + notmuch for search — two stores, neither knows labels or writes back. Weekly ritual: incremental backup + ad-hoc archive questions. Frustration: sync/search/analytics in three tools; none joins messages, threads, senders, sizes.

## Candidates (pre-cut)

| # | Name | Command | Persona | Source | Notes |
|---|------|---------|---------|--------|-------|
| 1 | Scheduled send queue | schedule send/list/cancel/edit/run | Sofia | (e) | user-vision, shipping scope |
| 2 | Sender leaderboard | senders | Marcus, Ken | (c) | local GROUP BY |
| 3 | Unsubscribe audit | unsub | Marcus | (b) | List-Unsubscribe mining |
| 4 | Storage hogs | storage | Marcus, Ken | (c) | sizeEstimate aggregation |
| 5 | Needs-reply tracker | followups --direction in | Priya, Marcus | (a) | |
| 6 | Outreach follow-up tracker | followups --direction out | Sofia | (a) | merged with #5 |
| 7 | Filters as code | filters diff/apply | Marcus | (b) | flat YAML, descoped from jsonnet |
| 8 | Sender dossier | sender report | Ken | (c) | merged into senders --sender |
| 9 | Drip send | schedule --spread | Sofia | (e) | killed: flag not feature |
| 10 | MCP credential import | auth import | Priya | (f) | killed: run-once |
| 11 | Retention rules | clean --rules | Marcus | (b) | killed: composable |
| 12 | Duplicate finder | dupes | Ken | (c) | killed: Gmail dedupes server-side |
| 13 | Label stats | analytics --type labels | Marcus | (c) | killed: framework analytics covers |
| 14 | Auth-results audit | authcheck | Ken | (b) | killed: niche |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|--------------|----------|
| 1 | Scheduled send queue (user-vision) | `schedule send --at/--in`, `schedule list/cancel/edit`, `schedule run --watch` | 9/10 | Sofia | hand-code | Stores RFC2822 payloads in a local `scheduled_sends` SQLite table and fires them through users.messages.send at due time with idempotency keys, no external dependencies | User Vision section (explicit request); brief workflow #4 notes "NO tool has this — API gap, UI-only feature" |
| 2 | Sender leaderboard | `senders --limit 20` (detail: `senders --sender user@example.com`) | 9/10 | Marcus, Ken | hand-code | GROUP BY from_email over the synced messages table computing count, unread, sum(sizeEstimate), first/last seen, entirely local | Brief workflow #5 "sender-frequency and storage analytics"; Product Thesis: gws/MCP servers have zero analytics |
| 3 | Unsubscribe audit | `unsub --min-count 10` | 7/10 | Marcus | hand-code | Reads stored List-Unsubscribe header values and joins them with per-sender volume/unread ratio from local SQLite to rank and emit unsubscribe targets | Gmail-specific content pattern (List-Unsubscribe RFC header); brief workflow #3 bulk-cleanup demand; no competitor touches it |
| 4 | Storage hogs | `storage --group-by sender` | 8/10 | Marcus, Ken | hand-code | Aggregates synced sizeEstimate by sender/year/attachment-presence locally and prints the matching `find` + `batchDelete --dry-run` cleanup commands | Brief workflow #3 "query old/large" mail; GYB-quota gap; data-layer storage-analytics joins |
| 5 | Follow-up tracker | `followups --direction in\|out --days 3` | 8/10 | Sofia, Marcus, Priya | hand-code | SQL window query over local threads+messages finding threads whose latest message author is (or is not) a sendAs address, aged past N days | Brief workflow #4 outreach + workflow #1 triage; per-thread fetch is the only live-API route (20 units/msg) |
| 6 | Filters as code | `filters diff --file filters.yaml`, `filters apply` | 7/10 | Marcus | hand-code | Diffs a flat YAML filter spec against users.settings.filters list output and executes the create/delete plan through the absorbed filter endpoints | Brief Table Stakes: "gmailctl-style diff/apply is differentiator territory"; gmailctl's popularity as a single-purpose tool |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Drip send (--spread) | A flag on the schedule queue, not a feature | Scheduled send queue |
| Sender dossier | Duplicate query engine; survives as `senders --sender <addr>` | Sender leaderboard |
| MCP credential import | Run once per machine, never weekly; auth-bootstrap doc material | none |
| Retention rules | Composable from absorbed find + batchModify/batchDelete --dry-run | Filters as code |
| Duplicate finder | Gmail dedupes by Message-ID server-side; near-empty output | Storage hogs |
| Label stats | Framework `analytics --type labels` already produces this | Sender leaderboard |
| Auth-results audit | Niche forensic need, no persona ritual | Unsubscribe audit |
| Needs-reply as separate command | Same SQL engine; merged into followups --direction | Follow-up tracker |
