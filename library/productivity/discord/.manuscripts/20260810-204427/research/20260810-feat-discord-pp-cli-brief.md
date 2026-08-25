# Discord CLI Brief (discord-pp-cli)

## API Identity
- Domain: Discord REST API v10 (guild management, messaging, moderation, webhooks)
- Users: Discord server admins, bot operators, moderators, agent workflows
- Data profile: guilds, channels, messages, members, roles, invites, webhooks. JSON over HTTPS. Official OpenAPI 3.1 spec, MIT license.
- Spec: `discord/discord-api-spec` `specs/openapi.json`, last commit 2026-08-06 (4 days old), actively maintained
- Spec checksum (sha256): f6cd197cbe47eeb6967a606980636b7fab3f72863255e17cd087cbbfe2f03bd2

## Reachability Risk
- None. Official API, official spec, no auth wall on spec fetch. 150 paths, 242 operations, all with operationId.
- Server: https://discord.com/api/v10
- Auth schemes in spec: BotToken (apiKey header, 203 ops), BotToken+OAuth2 (34 ops), OAuth2-only (5 ops, all user-scoped `/users/@me/...` entitlements/role-connections, not needed for server management)

## Top Workflows
1. Server overview and audit: which guilds the bot is in, channel trees, member counts, role inventory
2. Message ops: read channel history, send to channel/thread, edit, delete, pin, search
3. Moderation: list/inspect members, kick, ban, unban, manage roles, timeout
4. Channel/admin: create/archive channels, manage permissions, invites, webhooks
5. Agent integration: scripted status checks, alert posting, cross-server fan-out (MCP endpoint mirror)

## Table Stakes (competitor features)
- discord.py / discord.js / discord.sh: the incumbent SDKs. CLI-native competitors are scarce (no purpose-built Discord CLI in the public library; slack-pp-cli is the closest analog)
- Read channel messages, send message, list guilds/channels/members/roles, kick/ban, invite management
- Bot token auth via env var (DISCORD_BOT_TOKEN), same pattern as SLACK_BOT_TOKEN

## Data Layer
- Primary entities: guild, channel, message, member, role, invite, webhook
- Local store: SQLite mirror for guild/channel/member inventory + recent message history (offline `whois`, `guild snapshot`, `channel history` without re-fetch)
- Sync cursor: per-channel last-fetched message id; per-guild member snapshot timestamp
- FTS/search: message text index for local recall (same pattern as slack-pp-cli Archive Recall)

## ToS / Policy Summary (research done 2026-08-10)
Sources: Discord Developer Terms of Service (support-dev.discord.com article 8562894815383) + Discord Developer Policy (article 8563934450327) + official rate-limits docs.
- Bot account via official REST API = sanctioned use. The CLI only supports bot tokens (BotToken scheme), never user tokens.
- Self-bots / user-account automation are prohibited (Developer Policy: no automated user accounts; ToS bans account automation). CLI must refuse user tokens.
- Rate limits: global 10,000 invalid requests per 10 minutes; per-route buckets via X-RateLimit-Bucket header; 429s with X-RateLimit-Scope must be respected. CLI client needs bucket-aware retry/backoff.
- Prohibited: unsolicited DMs, bulk messaging, server membership inflation, selling API data, message content scraping for AI/ML training (Policy item 21), contacting users without permission.
- Monetization: requires Discord Monetization Terms approval; non-commercial use is fine.
- The CLI is a management tool, not a messaging spam tool. Message-send commands stay user-initiated single-target.

## Product Thesis
- Name: discord-pp-cli
- Why it should exist: every other platform has terminal-first tooling; Discord's official API is fully spec'd but only reachable through SDKs. A focused CLI gives admins and agents the same one-shot power slack-pp-cli gives Slack users: read, audit, and act without opening the app.

## Build Priorities
1. guild (list, info, channels tree) - highest gravity entity, 53 paths
2. channel (messages read/send, threads, pins) - 32 paths
3. member (list, info, kick, ban, unban, timeout) - moderation core
4. role (list, assign, unassign, info) - permissions management
5. invite + webhook (list, create, revoke) - admin utility
6. Local SQLite mirror + offline search (whois, history) - the novel transcendence feature vs raw SDK usage

## Novel Commands (transcendence candidates)
- `guild snapshot` - offline mirror of guild/channel/role inventory
- `whois <user>` - member card from local mirror (joined, roles, activity)
- `catch-up` - per-channel recent activity digest (slack-pp-cli parity)
- `channel history` - local message recall with FTS, no re-fetch
- `modlog`-style action history if API surface allows; otherwise skip

## CLI Shape (proposed)
- Binary: `discord-pp-cli`
- Category: productivity (matches slack)
- Auth: DISCORD_BOT_TOKEN env var, apiKey header (Authorization: Bot <token>)
- Commands: `guild`, `channel`, `member`, `role`, `invite`, `webhook`, `whois`, `catch-up`, `config`
- MCP: stdio endpoint mirror (like slack-pp-cli mcp_ready: full)
- Spec source: official (--spec-source official, --spec-url points at discord-api-spec)

## Generation Plan
- `cli-printing-press generate --spec C:/tmp/discord-openapi.json --name discord --category productivity --auth-preference BotToken --spec-source official --spec-url https://github.com/discord/discord-api-spec/blob/main/specs/openapi.json`
- Then hand-build transcendence: local SQLite mirror, catch-up, whois
- Then dogfood against a real bot token (live matrix) + shipcheck
- Then publish per printing-press-publish skill (fork, branch feat/discord, template body, dogfood proofs)
