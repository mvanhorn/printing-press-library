# skool CLI — Absorb Manifest

Sources absorbed: **SkoolAPI.com** (native REST, 11 endpoints) and **Apify `cristiantala/skool-all-in-one-api`** (33 actions). The Apify actor is the de-facto "best competitor"; we absorb its admin/write surface and beat it with a local SQLite mirror, offline querying, agent-native output, and an MCP server.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Create session (email+pw→session) | SkoolAPI sessions | `(generated endpoint) sessions create` + `(behavior in skool-pp-cli session create)` poll-to-active + cache | Auto-poll until active, cached to `~/.skool/session.json` |
| 2 | Get/delete session | SkoolAPI sessions | `(generated endpoint) sessions get` / `sessions delete` | — |
| 3 | Refresh / auto-refresh session | SkoolAPI sessions | `skool-pp-cli session refresh` | Auto-recreate on `authentication_error` mid-call |
| 4 | List posts (page/sort/category) | SkoolAPI posts | `skool-pp-cli post list` | Offline cache, `--json`, `--select`, group default from config |
| 5 | Find unanswered posts | Apify posts:getCommentsFull | `(behavior in skool-pp-cli post list --unanswered)` | Joins native posts with Apify comment counts; offline after sync |
| 6 | Draft/create post from markdown | Apify posts:create | `skool-pp-cli post draft <file.md>` | `--schedule` local queue, `--dry-run`, markdown body |
| 7 | List chats (inbox) | SkoolAPI chats | `skool-pp-cli chat inbox` | Newest-first, unread inferred via local read-state, `--json` |
| 8 | Get chat messages | SkoolAPI chats | `(generated endpoint) chats get` | Cursor pagination (message_id/limit) |
| 9 | Send chat message | SkoolAPI chats | `skool-pp-cli chat reply <id> <msg>` | Sends + marks read in one step |
| 10 | Mark chat read / unread | SkoolAPI chats | `skool-pp-cli chat read` / `chat unread` | — |
| 11 | Mark every chat read | SkoolAPI chats | `skool-pp-cli chat read-all` | Iterates inbox, `--dry-run` |
| 12 | List webhooks | SkoolAPI webhooks | `skool-pp-cli webhook list` | — |
| 13 | Create webhook | SkoolAPI webhooks | `skool-pp-cli webhook create` | event validation (post/comment/group_stats/chat_update) |
| 14 | Delete webhook | SkoolAPI webhooks | `skool-pp-cli webhook delete <id>` | — |
| 15 | Watch/tail webhook events | (new) | `skool-pp-cli webhook watch --events post,comment` | Registers webhook + runs local listener, tails events to terminal |
| 16 | List active members | Apify members:list | `(behavior in skool-pp-cli member list)` | Synced to local store |
| 17 | List pending applicants | Apify members:listPending | `(behavior in skool-pp-cli member list --pending)` | — |
| 18 | Approve member | Apify members:approve | `skool-pp-cli member approve <id>` | — |
| 19 | Reject member | Apify members:reject | `skool-pp-cli member reject <id>` | — |
| 20 | Batch-approve all pending | Apify members:batchApprove | `skool-pp-cli member approve-all` | `--dry-run`, count summary |
| 21 | Ban member | Apify members:ban | `skool-pp-cli member ban <id> --reason` | — |
| 22 | Export members CSV | Apify members:list | `skool-pp-cli member export` | CSV to stdout/file, from local mirror |
| 23 | Set Auto DM welcome | Apify groups:updateAutoDm | `skool-pp-cli autodm set <message>` | — |
| 24 | Publish course from markdown folder | Apify classroom:create* | `skool-pp-cli course publish <folder/>` | Walks folder → course/folders/pages, markdown→TipTap via actor |
| 25 | Group info / status | Apify groups:get | `skool-pp-cli group status` | member count, new-today (from mirror), recent posts |
| 26 | Update group description | Apify groups:updateDescription | `skool-pp-cli group set-description <text>` | — |
| 27 | Config get/set/show | (table stakes) | `skool-pp-cli config set/show` | `~/.skool/config.yaml` + `.env`, default group |
| 28 | Local mirror sync | (transcendence infra) | `skool-pp-cli sync` | Pulls posts+chats+members into SQLite |
| 29 | Raw SQL over mirror | (transcendence infra) | `skool-pp-cli sql "<query>"` | SELECT-only, like the linear CLI |
| 30 | Full-text search | (transcendence infra) | `skool-pp-cli search "<term>"` | FTS over posts + messages |
| 31 | Doctor / health | (table stakes) | `skool-pp-cli doctor` | checks secret, session active, apify token, db |

## Transcendence (only possible with our local-mirror approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| 1 | Unanswered-post queue with age | `post unanswered --older-than 24h` | hand-code | Joins synced posts with comment counts + timestamps; no single API call gives this |
| 2 | Chat response-time report | `chat response-time` | hand-code | Requires local message history to compute reply latency per conversation |
| 3 | Inbox triage (who's waiting on you) | `chat waiting` | hand-code | Local read-state + last-author analysis across all chats at once |
| 4 | Member onboarding funnel | `member funnel` | hand-code | Pending→approved→active counts over time, only from mirrored snapshots |
| 5 | Content calendar / posting cadence | `post cadence --weeks 4` | hand-code | Historical post timestamps aggregated locally |
| 6 | Daily standup digest | `digest today` | hand-code | One-shot: new members, unanswered posts, unread chats, recent activity — a cross-entity local join |

## Stubs
- None planned. `webhook watch` ships a real local HTTP listener; `course publish` and Apify-backed commands ship real Apify calls that return an actionable error if `APIFY_API_TOKEN` is unset (not a stub — honest auth-gating).

## Reachability / scope notes for Phase Gate
- Native `api.skoolapi.com` is Cloudflare-503 to anonymous probes (auth-edge artifact; see brief). No creds available → Phase 5 live testing skipped; verify against mocks/dry-run.
- `post list --unanswered` and the unanswered queue need comment counts, which native PostOut lacks → routed through Apify; degrade with actionable error when `APIFY_API_TOKEN` absent.
- `group status` "new today" / "recent posts" come from the local mirror; raw member count comes from Apify groups:get. Native `group_stats` is webhook-only.
