# pp-slack research addendum

Scope: ADDENDUM to `g:/My Drive/Cursor/workspace/evals/pp-slack-scoping-2026-05-11.md` (Phase 1.5a/a.5). Only delta findings beyond the 12 proposed verbs and known regime.

---

## Sources audited

| Source | URL | Lang / Stack | Last touched | License | Notes |
|---|---|---|---|---|---|
| rusq/slackdump | https://github.com/rusq/slackdump | Go | active 2026 | GPL-3.0 | Mature workspace dumper, v3 has SQLite backend + built-in MCP mode |
| korotovsky/slack-mcp-server | https://github.com/korotovsky/slack-mcp-server | Go | active 2026 | MIT | "Stealth" xoxc/xoxd browser-token MCP, GovSlack, smart history |
| modelcontextprotocol/servers-archived/src/slack | https://github.com/modelcontextprotocol/servers-archived/tree/main/src/slack | TypeScript | archived 2025 | MIT | Canonical reference MCP — 8 tools, xoxb only |
| AVIMBU/slack-mcp-server | https://github.com/AVIMBU/slack-mcp-server | TypeScript | 2025 | MIT | Minimal — only 2 of ~100 scopes implemented |
| piekstra/slack-mcp-server | https://github.com/piekstra/slack-mcp-server | TS | 2025 | MIT | Block Kit-focused MCP |
| kazuph/mcp-slack | https://github.com/kazuph/mcp-slack | TS | 2025 | MIT | Adds user resolution + Unicode normalization |
| atarukun/slack-mcp | https://github.com/atarukun/slack-mcp | TS | 2025 | MIT | "Production-ready" — broader scope coverage claimed |
| tuannvm/slack-mcp-client | https://github.com/tuannvm/slack-mcp-client | Go | 2026 | MIT | Inverse direction — Slack-bot-as-MCP-client (not relevant to pp-slack scope) |
| retrodigio/claude-channel-slack | https://github.com/retrodigio/claude-channel-slack | TS | 2026 | MIT | Socket Mode bridge — per-thread subagent persistence |
| agentskillexchange/skills/slack-digest-and-task-router | https://github.com/agentskillexchange/skills/blob/main/skills/slack-digest-and-task-router/SKILL.md | Markdown skill | 2026 | n/a | Built on Slack Bolt JS, mention-pattern task routing |
| ComposioHQ awesome-claude-skills "Slack Automation" | https://github.com/ComposioHQ/awesome-claude-skills | Markdown skill | 2026 | MIT | Messages, channels, search, reactions, threads, scheduling |
| takecy/slack-cli | https://github.com/takecy/slack-cli | Go | stale | MIT | Trivial send-message CLI |
| candrholdings/slack-cli | https://github.com/candrholdings/slack-cli | Node | stale | MIT | Token-env send-only |
| slacker (PyPI) | https://pypi.org/project/slacker/ | Python | maint mode | Apache-2.0 | Legacy full-API wrapper |
| slack-sdk (PyPI) | https://pypi.org/project/slack-sdk/ | Python | active 2026 | MIT | Successor to slackclient — official |
| slack-bolt (PyPI) | https://pypi.org/project/slack-bolt/ | Python | active 2026 | MIT | App framework (Events/Interactivity) — not a CLI |
| slack-conversation-export (npm) | https://www.npmjs.com/package/slack-conversation-export | Node | stale | MIT | "Export your participation" only, JSON |
| slack-history-export (npm) | https://www.npmjs.com/package/slack-history-export | Node | stale | MIT | Global-install token-based exporter |
| dcaslin/slack-export | https://github.com/dcaslin/slack-export | Node/TS | 2025 | MIT | Workspace-admin-required text+JSON+attachments dumper |
| Slack rate-limit changelog (May 2025) | https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/ | docs | 2025 | n/a | Authoritative limit list |
| dblock "AI Slop" post | https://code.dblock.org/2026/03/12/ai-slop-a-slack-api-rate-limiting-disaster.html | blog | 2026-03-12 | n/a | Real-world workaround pattern (cron + cap concurrent ops) |

No anthropics/claude-plugins-official Slack plugin exists; `claude-channel-slack` is the closest community equivalent.

---

## Competing CLIs (Slack workspace tools)

| Tool | Lang | Stars (approx) | Features | Worth absorbing? |
|---|---|---|---|---|
| **rusq/slackdump** | Go | 3k+ | `wiz` (interactive), `list`, `dump`, `export` (Mattermost + Standard Slack export modes), `archive` (SQLite or JSON+GZ), `convert` (transform formats), `emoji` (custom emoji download), `view` (built-in viewer for dumps), `search` (messages + files), `mcp` (built-in MCP server mode), `tools` (merge/dedupe/cleanup SQLite), `workspace` (multi-workspace config). xoxc+xoxd "EZ-Login 3000" browser auth, no admin needed, supports free-plan workspaces, incremental resume. | **YES** — incremental resume, SQLite backend, view command, dump-as-archive, multi-workspace config, and `convert` between formats are all novel relative to the scoping doc. |
| **takecy/slack-cli** (Go) | Go | <100 | Send message only | No |
| **candrholdings/slack-cli** (Node) | Node | <100 | Send message + token env | No |
| **dcaslin/slack-export** | Node/TS | <100 | Full workspace export (admin required) — text + JSON + attachments | No — admin-only, slackdump strictly superior |
| **slack-history-export** (npm) | Node | <100 | History to JSON, token auth | No |
| **slack-conversation-export** (npm) | Node | <100 | Exports participation only (not workspace) | No |
| **jedipunkz/slack-cli** | Go | <100 | Send message | No |

slackdump is the only mature competitor. Everything else is a send-message wrapper or single-purpose script.

---

## Competing MCP servers

| Server | Tools exposed | Source-extracted endpoints | Auth | Notes |
|---|---|---|---|---|
| **modelcontextprotocol/servers-archived/src/slack** | `slack_list_channels`, `slack_post_message`, `slack_reply_to_thread`, `slack_add_reaction`, `slack_get_channel_history`, `slack_get_thread_replies`, `slack_get_users`, `slack_get_user_profile` (8 total) | `conversations.list`, `conversations.info`, `chat.postMessage`, `reactions.add`, `conversations.history`, `conversations.replies`, `users.list`, `users.profile.get` | `Authorization: Bearer ${SLACK_BOT_TOKEN}` (xoxb only), `Content-Type: application/json`. Cursor-pagination via `URLSearchParams.append("cursor", cursor)`, response includes `response_metadata.next_cursor`. | The canonical reference — archived. Bot-token only. No DMs, no search.messages, no scheduled messages, no file ops. |
| **korotovsky/slack-mcp-server** | Read: `conversations_history`, `conversations_replies`, `conversations_search_messages`, `channels_list`, `users_search`, `usergroups_list`, `conversations_unreads`, `usergroups_me`. Write (default-off): `conversations_add_message`, `reactions_add`/`remove`, `conversations_mark`, `usergroups_create`/`update`/`users_update`. | Implied: `conversations.history`, `conversations.replies`, `search.messages`, `conversations.list`, `usergroups.list/create/update`, `usergroups.users.list/update`, `client.counts`, `conversations.info`, `conversations.mark`, `reactions.add/remove` | Three modes: stealth (`SLACK_MCP_XOXC_TOKEN`+`SLACK_MCP_XOXD_TOKEN` browser cookies), user OAuth (`SLACK_MCP_XOXP_TOKEN`), bot (`SLACK_MCP_XOXB_TOKEN`). | Notable novel features: smart-history flexible date ("1d","7d","1m","90d"), GovSlack routing (`slack-gov.com`), `client.counts` for unreads, channel `#name` lookup, user-context enrichment, DM/Group DM `@username_dm`, stdio+SSE+HTTP transports, proxy support, on-disk users/channels cache. |
| **AVIMBU/slack-mcp-server** | List users + post message (2 only) | `users.list`, `chat.postMessage` | `xoxb` bot token + `SLACK_TEAM_ID` | Skeleton — ~100 unimplemented scopes. |
| **piekstra/slack-mcp-server** | Block Kit message construction | `chat.postMessage` + Block Kit | xoxb | Useful only if pp-slack needs Block Kit composer support. |
| **kazuph/mcp-slack** | Standard tools + user resolution + Unicode normalization | Standard set | xoxb | Unicode normalization (JP-style) is a niche borrow. |
| **rusq/slackdump `mcp` subcommand** | Read-only over local archive | none (queries local SQLite) | none (post-dump) | Novel: serves dumped data as MCP — useful pattern for `pp-slack sync` + serve. |

### Auth pattern summary (verbatim from source reads)

```typescript
// servers-archived/src/slack/index.ts
this.botHeaders = {
  Authorization: `Bearer ${botToken}`,
  "Content-Type": "application/json",
};
```

All API calls hit `https://slack.com/api/<method>` over HTTPS. GovSlack variant (korotovsky) substitutes `https://slack-gov.com/api/<method>`.

---

## SDK wrappers (npm/PyPI)

| Package | Lang | Notable methods | Auth shape |
|---|---|---|---|
| `slack-sdk` (PyPI) | Python | `WebClient.conversations_history`, `chat_postMessage`, `users_list`, full method coverage. Successor to deprecated `slackclient`. Async via `AsyncWebClient`. | `WebClient(token="xoxb-...")` |
| `slack-bolt` (PyPI) | Python | App framework — `@app.event("app_mention")`, `@app.command`, Socket Mode handler | OAuth + signing secret |
| `slacker` (PyPI) | Python | Legacy full-API wrapper, maint-mode | Token string |
| `slackclient` (PyPI) | Python | Deprecated — replaced by slack-sdk | — |
| `@slack/web-api` (npm, official) | Node/TS | `WebClient.chat.postMessage`, full method tree | Token string |
| `@slack/bolt` (npm, official) | Node/TS | App framework + Socket Mode | OAuth + signing secret |
| `@slack/cli-test` (npm) | Node | Testing harness for Slack's own Deno CLI | — |
| `slack-go/slack` | Go | The de-facto Go client — `Client.PostMessage`, `GetConversationHistory`, websocket RTM, Socket Mode | `slack.New(token)` |

Implication for pp-slack: `slack-go/slack` is the obvious dependency; cross-check OpenAPI surface against its method names to catch any nullable-field decoding bugs (community has had issues with `is_member`, `latest_reply` types).

---

## Features the scoping doc missed

Filtered to non-duplicative additions only. Each is something none of the 12 proposed verbs (`digest`, `customer-intel`, `drift`, `dms-summary`, `dormant`, `attention`, `who-said`, `thread-summary`, `post`, `schedule`, `channel-find`/`user-find`, `sync`) covers.

1. **`export` to Slack Standard / Mattermost format** — Slackdump's killer feature. One zip that imports into Slack, Mattermost, Discord-migration tools, e-discovery platforms. Compliance/legal-hold use case AtomChat will hit eventually. (Source: slackdump README.)
2. **`convert` between local formats** — JSON+GZ ↔ SQLite ↔ Standard export ↔ Mattermost. Useful once `sync` lands.
3. **`view`** — local browser/TUI for a downloaded archive. Slackdump ships one. Pairs with `pp-slack sync` if we ever go local-first.
4. **`emoji`** — download custom workspace emoji (image files + name map). Niche but cheap, and zero competing skill has it.
5. **`workspace` config command** — multi-workspace switching (xoxc tokens per workspace, default profile). Pp-slack will need this once Brasil/Latam team uses it across orgs.
6. **`unreads` with priority sort** — korotovsky's `conversations_unreads` uses `client.counts` (browser-token-only) to surface "DMs > partner channels > internal channels" sort. Different shape than `digest`, which is window-based. Unreads = "what should I read RIGHT NOW".
7. **`@username_dm` resolution** — name-based DM addressing for the post/schedule verbs. Today's MCP forces channel-IDs.
8. **`#name` channel lookup with on-disk cache** — most MCPs force you to know `C0123ABC` IDs. Korotovsky caches and resolves names. Necessary UX for cron jobs that reference `#csm-signals`, `#churnsales`, etc. without hardcoding IDs.
9. **`reactions add/remove`** — proposed `post` doesn't cover reactions. Required for narrative-engine acknowledgement loops (alert-engine reacts to its own message when downstream action completes).
10. **`mark` (mark-as-read)** — required to wire pp-slack into the daily DM-summary workflow without polluting unread state in the human's Slack client.
11. **`usergroups` (list / membership)** — `<!subteam^S0123>` mention rendering, on-call rotations. AtomChat uses these for `@csm-emea`, `@bdr-latam`. Today the digest skill mis-renders subteam mentions.
12. **`socket-mode` listener** — `retrodigio/claude-channel-slack` shows Socket Mode is achievable in a CLI process. Out of pp-slack core scope but worth a stub command so it isn't impossible to add later.
13. **Stealth-mode auth (xoxc + xoxd)** — significant. Bot tokens (xoxb) require workspace admin to install a Slack app with scopes; xoxc+xoxd come from a logged-in browser session, work with zero workspace-admin involvement, and can read DMs the user themselves can read. Adding both auth modes doubles addressable surface area at low cost.
14. **`search.messages` (user-token only)** — bot tokens can't call it; xoxp/xoxc can. Proposed `who-said` likely needs this; flag it explicitly. (Source: korotovsky README "search.messages — unavailable for bot tokens".)
15. **`files.upload`/`files.list`** — none of the 12 verbs touches file uploads. AtomChat's narrative engine occasionally needs to attach a CSV/PNG to a Slack message. Cheap add.
16. **`chat.scheduleMessage` vs Slack-side reminder** — proposed `schedule` should specifically use `chat.scheduleMessage` (server-side, survives client restart) and not local cron. Worth pinning in the contract.
17. **Mattermost-export format** — same data, different consumer. If AtomChat ever needs to migrate Slack -> Mattermost or escrow conversations to an e-discovery vendor.

---

## DeepWiki insights

Skipped — Slack itself is closed-source. No DeepWiki-style architectural inspection applies. The closest analog is reading slackdump's own architecture, which is already captured above (chunk-file backend → SQLite backend → archive-as-canonical, on-the-fly conversion).

---

## Reachability / latent-bug findings

1. **The 1-req/min cap is real and tightening.** May 29, 2025 hit new non-Marketplace installs. The scoping doc's note about a March 2026 enforcement date for *existing* installs is NOT explicitly stated in the Slack changelog itself (per the May 2025 doc I fetched) — but is widely reported in community blogs. **Confirm enforcement date independently before launch.** Internal customer-built apps remain at 50+ req/min × 1000 obj/page. (Source: https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps/)
2. **Affected endpoints are exactly two**: `conversations.history`, `conversations.replies`. `search.messages` is NOT in that changelog. So a user-token search-first design dodges the worst of the cap. (Same source.)
3. **`limit: 100` silently capped at 15.** Multiple GitHub issues confirm Slack silently caps the page size for affected apps regardless of what the client requests. Pp-slack must surface this — if the user passes `--limit 100` and only 15 come back, the CLI should log a warning, not silently truncate. (Source: https://github.com/orgs/community/discussions/162325)
4. **`Retry-After` header is authoritative.** Community workaround is exponential backoff WITH jitter, but always respect `Retry-After` first. (Source: https://api.slack.com/apis/rate-limits)
5. **Global cross-endpoint rate caps exist.** dblock's post documents that even non-throttled endpoints can hit a global cap if other parts of the app are spinning. Implication: pp-slack's local SQLite cache (sync once, query offline) is the only sustainable architecture for cron workloads — same conclusion the scoping doc reached, now confirmed by a primary post-mortem. (Source: https://code.dblock.org/2026/03/12/ai-slop-a-slack-api-rate-limiting-disaster.html)
6. **slackdump v3 uses SQLite as primary storage with 5× faster conversion than its old chunk-file format.** If pp-slack `sync` lands, copy the schema rather than invent one. (Source: slackdump changelog.)
7. **xoxc tokens rotate on browser logout.** Plan for token-refresh UX, not a one-time setup.

---

## Recommendations for the absorb manifest

Concrete additions for the Phase 2 absorb table. Each row maps a single feature/pattern to a single source.

| Feature to absorb | Source | Why |
|---|---|---|
| `export` verb producing Slack Standard + Mattermost zip formats | `rusq/slackdump` (export, convert) | Compliance/legal-hold use case, zero competing CLI offers it, format spec is stable. |
| `archive` verb backed by SQLite (one DB file per workspace) | `rusq/slackdump` v3 backend | 5× faster than JSON+GZ, enables FTS5 offline queries. Direct fit for `pp-slack sync`. |
| Stealth auth: `xoxc` + `xoxd` token mode in addition to `xoxp`/`xoxb` | `korotovsky/slack-mcp-server` | Zero workspace-admin friction, reads DMs the user can read. Roughly doubles useful surface area. |
| `#channel-name` and `@username` resolution with on-disk cache | `korotovsky/slack-mcp-server` | Required for cron jobs and L10/digest configs that hard-code channel names. |
| `unreads` verb using `client.counts` (browser-token only) with priority sort | `korotovsky/slack-mcp-server` | Different shape from `digest` (`attention`/window-based) — unreads = "right now". |
| `usergroups` list + membership commands rendering `<!subteam^S…>` mentions correctly | `korotovsky/slack-mcp-server` | AtomChat uses `@csm-emea`, `@bdr-latam` — current digest mis-renders these. |
| `chat.scheduleMessage` (server-side scheduling) for the `schedule` verb, not a local cron | Slack API + scoping doc note | Survives client restart, no daemon required, idempotent. |
| Mandatory warning when page-size is silently capped at 15 | GitHub community discussions #162325 | Latent-bug-class — user passes `--limit 100`, gets 15, no error. |
| `mark` (conversations.mark) so cron reads don't pollute unread state for the human | `korotovsky/slack-mcp-server` | Required to make `digest` and `attention` non-destructive. |

Out of scope for v1 but worth a stub: `view`, `socket-mode`, `emoji`, `files.upload`, `convert`. Implement after `sync` proves out.
