# AdminByRequest Absorb Manifest

## Ecosystem Survey (Step 1.5a)
Searches conducted: GitHub for "adminbyrequest" CLI/MCP/SDK, npm/PyPI for wrappers, AbR's own prebuilt integrations.

**Findings:**
- AbR official prebuilt integrations (not open-source CLIs): Splunk app, Microsoft Sentinel pack, Power BI templates, Jira/ServiceNow/Teams/Slack connectors. These are vertical pipes from AbR -> specific systems, not general-purpose CLIs.
- No mature open-source CLI for AbR API exists at the time of research.
- No published MCP server for AbR.
- No npm or PyPI SDK discovered.

**Implication:** Absorb target is the AbR portal UI's read-only functions and the integration pipes' transport behavior. There is no open-source incumbent CLI to match feature-for-feature; we are setting the bar.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List audit log entries | AbR portal AuditLog view | auditlog list w/ startid/take/scandetails | --json, --csv, --select, offline SQLite mirror, FTS |
| 2 | Filter audit log | AbR portal filters | auditlog list --user --computer --type --status | All filters offline via SQL composability |
| 3 | Delta sync audit log | AbR Splunk integration | auditlog delta --since <ticks> and sync | Bookmark cursor in local store; resumable |
| 4 | List events | AbR portal Events view | events list --code N | Offline; FTS on eventText |
| 5 | List inventory devices | AbR portal Inventory | inventory list | Local store; agent-version drift queries |
| 6 | Inventory + software | wantsoftware=1 flag | inventory list --software | Persist software inventory locally |
| 7 | Inventory + hardware | wanthardware=1 flag | inventory list --hardware | Persist hardware specs locally |
| 8 | Inventory by owner (SID/name) | docs spec | inventory list --owner-sid --owner-name | Composable with offline join |
| 9 | List pending requests | AbR portal Requests | requests list --status pending | Pipe to approve/deny |
| 10 | Approve elevation | AbR portal action | requests approve <id> --reason | --dry-run, batch via --stdin |
| 11 | Deny elevation | AbR portal action | requests deny <id> --reason | --dry-run, batch via --stdin |
| 12 | List approved/denied history | AbR portal status filter | requests list --status approved/denied | Offline mirror |
| 13 | Generate offline PIN | AbR portal PIN dialog | inventory pin <computer> --type challenge | One-line; no portal click-through |
| 14 | Quota awareness | AbR docs (100k/day) | Local quota tracker + quota show | Counts every call; warns at 80% / 95% / 99% |

Every row above ships fully (no stubs).

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|------------------------|-------|
| 1 | Repeat-requestor detection | requests repeat-offenders --window 30d | Requires cross-time aggregation over local store; portal cannot show top-N users by request count | 8/10 |
| 2 | Agent-version compliance drift | inventory drift --client-version 8.7.2 | Compare local inventory snapshot vs target version; flag endpoints with stale AbR agent | 7/10 |
| 3 | Audit-to-event correlation | correlate <auditlog-id> --window 5m | Join an audit entry to events on the same computer within plus/minus N minutes; SQL across two tables, no API call does this | 8/10 |
| 4 | Denied-reason distribution | requests denied-reasons --top 20 | Tokenize deniedReason strings from local store; useful for compliance review | 6/10 |
| 5 | Quota usage forecast | quota forecast --days 1 | Track call rate; predict whether daily 100k will be hit; suggest sync schedule changes | 7/10 |
| 6 | Compliance report (CSV/markdown) | report compliance --since 2026-01-01 --user <name> | Render audit entries for a user/window into a report fixture suitable for auditors | 6/10 |
| 7 | Endpoint risk score | inventory risk-score | Score each endpoint by elevation frequency, admin-on-machine count, stale-agent flag | 5/10 |

All transcendence rows above score >= 5/10 and are included in novel_features for README/SKILL rendering.

## Stubs
None. Every approved row ships fully.

## Source Priority
Single-source CLI (AbR official API only). Multi-source priority gate does not apply.
