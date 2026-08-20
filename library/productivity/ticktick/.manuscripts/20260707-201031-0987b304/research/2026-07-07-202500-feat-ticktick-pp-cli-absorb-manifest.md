# TickTick Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List projects | Open API /project (jacepark12 MCP) | (generated endpoint) projects list | offline mirror, --json/--select |
| 2 | Get project with tasks | Open API /project/{id}/data | (generated endpoint) projects data | offline |
| 3 | Create/update/delete/complete task | Open API + V2 batch (all MCPs) | (generated endpoint) tasks create/update/delete/complete | safe field whitelist, --dry-run |
| 4 | Batch task operations | V2 /batch/task (tick-mcp source) | (generated endpoint) batch task | etag-carrying, never sends kind |
| 5 | Get task by id | V2 (tick-mcp) | (generated endpoint) tasks get | full JSON incl. kind/etag/childIds |
| 6 | Search/filter tasks | jen6/ticktick-mcp filters | ticktick-pp-cli search | offline FTS, regex |
| 7 | List habits | V2 /api/v2/habits (OliverStoll, karbassi) | (generated endpoint) habits list | offline |
| 8 | Habit checkins query | V2 /api/v2/habitCheckins/query | (generated endpoint) habits checkins | offline history |
| 9 | Upsert habit checkin | V2 (OliverStoll write path) | (generated endpoint) habits checkin | correct habit-ID handling (fixes 7/2 MCP bug) |
| 10 | Focus/pomodoro records | V2 (ticktick-py pomo manager) | (generated endpoint) focus list | offline stats source |
| 11 | Tags list/manage | V2 (ticktick-py tags manager) | (generated endpoint) tags list | offline |
| 12 | Completed tasks by date | V2 /api/v2/project/all/completed (tick-mcp) | (generated endpoint) completed list | week-in-review source |
| 13 | Project groups | V2 (claude.ai MCP parity) | (generated endpoint) project-groups list | offline |
| 14 | Incremental sync | V2 /batch/check/0 (ticktick-py state) | ticktick-pp-cli sync | SQLite mirror + FTS |
| 15 | User profile/preferences | V2 (claude.ai MCP get_user_preference) | (generated endpoint) user preferences | timezone-aware date handling |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Persona | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|---------|--------------|------------------------|------------------|
| 1 | Safe daily-note edit | note edit | 10/10 | Ritualist, Agent | hand-code | Encodes the corruption-proof update contract (field whitelist, never `kind`, carries etag/isAllDay); absorbs `--append` timestamped logging | Use this command for editing the daily note's content safely. Do NOT use it for generic task field updates; use 'tasks update' instead. |
| 2 | Agenda | agenda | 9/10 | Ritualist, Agent | hand-code | Drain-first local join: today's tasks + habits w/ checkin state + focus records in one call | Use this command for a single day's snapshot. Do NOT use it to gather multi-day review data; use 'review' instead. |
| 3 | Week-in-review data pack | review --since 7d | 9/10 | Ritualist, Agent | hand-code | Local SQLite join over completed tasks, daily notes, focus, checkins → one structured pack | Use this command to gather a week's raw review data. Do NOT use it for a single day; use 'agenda' instead. |
| 4 | Habit streaks | habits streaks | 8/10 | Dana, Ritualist | hand-code | Streak math over synced checkins — no ecosystem tool computes this | none |
| 5 | Focus stats | focus stats --since 7d | 7/10 | Dana, Ritualist | hand-code | Per-day/per-project focus aggregation from local mirror | none |
| 6 | Auth doctor | doctor | 7/10 | Ritualist, Dana | hand-code (extends generated doctor) | Dual-tier V1+V2 token validation, UA-header check, Google-SSO cookie fallback guidance | none |

Killed candidates + customer model: see 2026-07-07-202500-novel-features-brainstorm.md
