# Slack CLI Absorb Manifest — reprint run 20260801-150754-271fdf29

## Sources Analyzed
1. **Official Slack MCP server** (`docs.slack.dev/ai/slack-mcp-server`) — hosted, admin-governed: search, send, canvases, profiles
2. **korotovsky/slack-mcp-server** (~1.7k★, TS) — stealth `xoxc`/`xoxd` mode, DMs, group DMs, smart history, caching, proxy
3. **slackapi/slack-skills-plugin** — official Claude Code plugin (MCP + Block Kit skills)
4. **slackapi/slack-cli** — official, app scaffold/deploy only (different niche)
5. **rockymadden/slack-cli** (Bash) — messaging, uploads, status, reminders, DND, pipe-friendly
6. **shaharia-lab/slackcli** (TS/Bun) — AI-friendly JSON/table/text, canvas
7. **tumf/slack-rs** (Rust) — OAuth, secure token storage
8. **piekstra/slack-chat-api**, **lox/slack-cli**, **candrholdings/slack-cli**, npm one-shot senders
9. **slackapi/python-slack-sdk** — official SDK (rate-limit handling complaints: #287, #436, #960, #1693)
10. **slackapi/slack-api-specs** — archived 2021 OpenAPI 2.0, pruned 174 → 91 paths

## Spec Composition
Curated 62-endpoint internal YAML **merged with** a pruned copy of the archived official OpenAPI 2.0. Dropped families: `admin.*` (56, Enterprise Grid only), `apps` (8), `calls` (6), `views` (4), `oauth` (3), `workflows` (3), `dialog`, `migration`, `rtm`. Result: **91 endpoints** across conversations(18), files(13), users(12), chat(10), usergroups(7), dnd(5), reminders(5), team(5), reactions(4), pins(3), stars(3), auth(2), api(1), bots(1), emoji(1), search(1).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Send message to channel | rockymadden, piekstra MCP | (generated endpoint) messages post_message | `--dry-run`, `--json`, thread_ts, stdin |
| 2 | Send DM | korotovsky MCP | (generated endpoint) messages post_message | Channel-or-user target, dry-run |
| 3 | Update message | rockymadden, piekstra MCP | (generated endpoint) messages update_message | `--dry-run` |
| 4 | Delete message | piekstra MCP | (generated endpoint) messages delete_message | `--dry-run` |
| 5 | Schedule message | Slack API | (generated endpoint) messages schedule_message | Typed exit codes |
| 6 | List scheduled messages | Slack API | (generated endpoint) messages list_scheduled | `--json`, `--select` |
| 7 | Get permalink | Slack API | (generated endpoint) messages get_permalink | Composable into other commands |
| 8 | Read channel history | korotovsky MCP, lox | (generated endpoint) conversations history | Offline after sync; cursor pagination fixed |
| 9 | Read thread replies | korotovsky MCP, shaharia-lab | (generated endpoint) conversations replies | Offline after sync |
| 10 | List channels | all tools | (generated endpoint) conversations list | Offline, `--json`, `--csv`, FTS on names |
| 11 | Channel info | piekstra MCP, lox | (generated endpoint) conversations get | Offline after sync |
| 12 | Channel members | piekstra MCP | (generated endpoint) conversations members | Offline join to users |
| 13 | Create channel | piekstra MCP | (generated endpoint) conversations create | `--dry-run` |
| 14 | Archive channel | piekstra MCP | (generated endpoint) conversations archive | `--dry-run` |
| 15 | Unarchive channel | piekstra MCP | (generated endpoint) conversations unarchive | `--dry-run` |
| 16 | Invite to channel | piekstra MCP | (generated endpoint) conversations invite | `--dry-run` |
| 17 | Set channel topic | piekstra MCP | (generated endpoint) conversations set_topic | `--dry-run` |
| 18 | Set channel purpose | piekstra MCP | (generated endpoint) conversations set_purpose | `--dry-run` |
| 19 | Mark channel read | korotovsky MCP | (generated endpoint) conversations mark | `--dry-run` |
| 20 | List users | all tools | (generated endpoint) users list | Offline, filterable |
| 21 | User info | lox, piekstra MCP | (generated endpoint) users get | Offline after sync |
| 22 | Lookup user by email | Slack API | (generated endpoint) users lookup_by_email | Feeds `users whois` |
| 23 | Get presence | rockymadden | (generated endpoint) users get_presence | `--json` |
| 24 | Set presence | rockymadden | (generated endpoint) users set_presence | `--dry-run` |
| 25 | Get profile | Slack API | (generated endpoint) users profile_get | `--select` |
| 26 | Set profile / status | rockymadden | (generated endpoint) users profile_set | `--dry-run` |
| 27 | Search messages | korotovsky MCP, lox | (generated endpoint) search_api messages | Live path; user token only — bot tokens get `not_allowed_token_type` |
| 28 | Add reaction | rockymadden, korotovsky | (generated endpoint) reactions add | `--dry-run` |
| 29 | Remove reaction | rockymadden | (generated endpoint) reactions remove | `--dry-run` |
| 30 | Get reactions for item | Slack API | (generated endpoint) reactions get | `--json` |
| 31 | List reactions | Slack API | (generated endpoint) reactions list | Offline after sync |
| 32 | File upload | rockymadden, piekstra MCP | (generated endpoint) files upload | stdin pipe, `--dry-run` |
| 33 | File list | rockymadden | (generated endpoint) files list | Offline, filterable |
| 34 | File info | rockymadden | (generated endpoint) files get | `--select` |
| 35 | File delete | Slack API | (generated endpoint) files delete | `--dry-run` |
| 36 | Reminders add/list/complete/delete/info | rockymadden | (generated endpoint) reminders add | User token only — bot gets `not_allowed_token_type` |
| 37 | Pins add/list/remove | Slack API | (generated endpoint) pins list | `--dry-run` on mutations |
| 38 | Stars add/list/remove | Slack API | (generated endpoint) stars list | User token only — bot gets `not_allowed_token_type` |
| 39 | DND snooze/end/info | rockymadden | (generated endpoint) dnd get | `--dry-run` on mutations |
| 40 | Usergroup list/create/update | korotovsky MCP | (generated endpoint) usergroups list | `--dry-run` |
| 41 | Usergroup members list/update | korotovsky MCP | (generated endpoint) usergroups users_list | `--dry-run` |
| 42 | Emoji list | Slack API | (generated endpoint) emoji list | Custom emoji + aliases |
| 43 | Team info | piekstra MCP | (generated endpoint) team get | `--json` |
| 44 | Team access logs | Slack API | (generated endpoint) team access_logs | Paid-plan gated; honest error |
| 45 | Team billable info | Slack API | (generated endpoint) team billable_info | `--json` |
| 46 | Auth test / identity | all tools | (generated endpoint) auth_api test | Feeds `doctor` |
| 47 | Bot info | Slack API | slack-pp-cli bots | `--json` |
| 48 | users.conversations membership | Slack API | (generated endpoint) users conversations | Offline join |
| 49 | Sync to SQLite | slacrawl | slack-pp-cli sync | Cursor pagination + `ok:false` detection |
| 50 | Offline FTS5 search | slacrawl | slack-pp-cli search | Works on bot token where `search.*` does not |
| 51 | Raw SQL queries | slacrawl | slack-pp-cli sql | Read-only, SELECT-gated |
| 52 | Analytics rollups | — | slack-pp-cli analytics | `--group-by`, replaces prior `trends` |
| 53 | Tail / poll a resource | slacrawl | slack-pp-cli tail | `--follow`, `--interval` |
| 54 | Doctor diagnostics | slacrawl | slack-pp-cli doctor | Reports token type → reachable families |
| 55 | Agent context | — | slack-pp-cli agent-context | Command-tree introspection |
| 56 | Structured output everywhere | shaharia-lab | (behavior in slack-pp-cli sync) `--json`, `--agent`, `--select`, `--compact`, `--csv`, `--quiet`, `--plain` on every command | Agent-native by default |
| 57 | Reachability honesty | python-slack-sdk (negative) | (behavior in slack-pp-cli sync) typed `RateLimitError` distinct from empty results | Fixes the SDK's 6-year complaint class |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Archive Recall | archive recall | hand-code | 10/10 | Local FTS5 over messages that outlive Slack's 90-day wall, with thread context and resolved names; works on a bot token where `search.*` returns `not_allowed_token_type` | Use this command to find messages in the local archive, including messages older than Slack's 90-day retention wall, with thread context and resolved display names. Do NOT use this command for a quick single-resource lookup of channels or users; use 'search' with --type instead. |
| 2 | Catch-up | catchup | hand-code | 9/10 | Single local pass joining messages + conversations + users + usergroup membership to compute obligation, not volume — collapses a 4-command chain into one | Use this command for "what happened while I was away" across channels — new volume, mentions of you, and threads still awaiting your reply — over a time window. Do NOT use this command to profile a named third party's posting behavior; use 'users activity' instead. |
| 3 | Archive Coverage | archive coverage | hand-code | 8/10 | Aggregates per-channel ts min/max/count and diffs synced ranges to expose gaps — the mirror is only trustworthy if its coverage is inspectable | none |
| 4 | Stale Thread Radar | threads stale | hand-code | 8/10 | Groups local messages by thread_ts and ranks by age where no reply follows a question-shaped parent; thread reply-ownership has no API equivalent | Use this command to list unanswered threads across the whole archive ranked by age since last reply. Do NOT use this command for a personal since-I-was-away summary; use 'catchup' instead. |
| 5 | Channel Health | health | hand-code | 7/10 | Per-channel aggregate (messages/day, distinct posters, median first-reply latency, idle days) comparable across channels; non-admin leads have no other route | Use this command to score and compare channels by volume, distinct posters, median first-reply latency, and idle days; --dying filters to archive candidates. Do NOT use this command for raw grouped counts over a time bucket; use 'analytics --type messages --group-by channel' instead. |
| 6 | User Activity | users activity | hand-code | 6/10 | Cross-channel per-user rollup from local messages + reactions | Use this command to profile where a single named person is active across channels and threads. Do NOT use this command for your own since-I-was-away summary; use 'catchup' instead. |
| 7 | Identity Card | users whois | hand-code | 6/10 | Resolves any identifier form against local users, joins messages for shared channels and last-seen; collapses the agent per-ID resolution loop | Use this command to resolve an opaque Slack ID, @handle, or email into one identity card with shared channels, timezone, DND state, and last-seen. Do NOT use this command when you need the raw profile payload; use 'users info' instead. |

**Hand-code commitment: 7 of 7.** No transcendence row is `spec-emits`.

## Dropped prior features (reprint verdicts — override available at the gate)
`response-times`, `trends`, `channels quiet` (folded into `health --dying`), `funny`, plus never-built `network`, `daemon`, `notifications`. `digest` → renamed `catchup`; `archive-search` → reframed as `recall`.

## Stubs
None. Every row above is shipping scope.
