# v0 API CLI — Absorb Manifest

Run: 20260804-232309-9bd8d5ce | API: v0 API v2 | Base: https://api.v0.dev/v2 | Auth: Bearer ($V0_API_KEY)

## Absorbed (match or beat everything that exists)

Sources scanned: community `v0-cli` (npm 1.1.6, v1 API — chat create/list/get/message/update/delete/favorite, project, hooks, deploy, vercel, config, output json|yaml|table), official `v0` npm SDK (v3.0.2, v2 — chats CRUD/stream/async, messages, files, mcp-servers, hooks, settings, deploy, vercel-project, error envelope), official v0 MCP server + @v0-sdk/ai-tools, create-v0-sdk-app template.

| # | Feature | In v0-cli | In v0 SDK/MCP | Our Implementation |
|---|---------|-----------|---------------|---------------------|
| 1 | Create chat (sync) | yes | yes | (generated endpoint) `v0-pp-cli chats create` |
| 2 | Create chat (async, poll messageId) | no | yes | (generated endpoint) `v0-pp-cli chats create-async` |
| 3 | Create chat streaming (SSE) | no | yes | `v0-pp-cli chats create-stream` (hand-wrap SSE capture) |
| 4 | Create chat from files | no | yes | (generated endpoint) `v0-pp-cli chats create-from-files` |
| 5 | Create chat from repo | no | yes | (generated endpoint) `v0-pp-cli chats create-from-repo` |
| 6 | Create chat from ZIP | no | yes | (generated endpoint) `v0-pp-cli chats create-from-zip` |
| 7 | List chats (cursor, filters) | yes | yes | (generated endpoint) `v0-pp-cli chats list` |
| 8 | Get chat | yes | yes | (generated endpoint) `v0-pp-cli chats get` |
| 9 | Update chat (title/privacy/metadata) | yes | yes | (generated endpoint) `v0-pp-cli chats update` |
| 10 | Delete chat | yes | yes | (generated endpoint) `v0-pp-cli chats delete` |
| 11 | Duplicate chat | yes (fork) | yes | (generated endpoint) `v0-pp-cli chats duplicate` |
| 12 | Send message (sync) | yes | yes | (generated endpoint) `v0-pp-cli messages send` |
| 13 | Send message (async) | no | yes | (generated endpoint) `v0-pp-cli messages send-async` |
| 14 | Send message (streaming SSE) | no | yes | (generated endpoint) `v0-pp-cli messages send-stream` |
| 15 | List messages (cursor) | yes | yes | (generated endpoint) `v0-pp-cli messages list` |
| 16 | Get message | yes | yes | (generated endpoint) `v0-pp-cli messages get` |
| 17 | Stop in-flight message | yes | yes | (generated endpoint) `v0-pp-cli messages stop` |
| 18 | Resume chat stream | no | yes | (generated endpoint) `v0-pp-cli chats resume-stream` |
| 19 | Resolve task (sync/async/stream) | no | yes | (generated endpoint) `v0-pp-cli messages resolve-task` |
| 20 | Get chat files | yes | yes | (generated endpoint) `v0-pp-cli chats files` |
| 21 | Update chat files | yes | yes | (generated endpoint) `v0-pp-cli chats update-files` |
| 22 | Download chat files (archive) | yes | yes | (generated endpoint) `v0-pp-cli chats download-files` |
| 23 | Get preview URL + token | no | yes | (generated endpoint) `v0-pp-cli chats preview` |
| 24 | Deploy chat to Vercel | yes | yes | (generated endpoint) `v0-pp-cli chats deploy` |
| 25 | Create Vercel project link | yes | yes | (generated endpoint) `v0-pp-cli chats vercel-project` |
| 26 | MCP servers CRUD (limit 10/user) | no | yes | (generated endpoint) `v0-pp-cli mcp-servers create/get/list/update/delete` |
| 27 | Webhooks CRUD (events chat.*/message.*) | yes | yes | (generated endpoint) `v0-pp-cli hooks create/get/list/update/delete` |
| 28 | Trusted preview hosts settings | no | yes | (generated endpoint) `v0-pp-cli settings preview-hosts get/set` |
| 29 | Auth config: V0_API_KEY / --api-key / saved config | yes | n/a | (generated) config + doctor |
| 30 | Output json/yaml/table | yes | n/a | (generated) --json/--csv/--select + human table |
| 31 | Restore message (rollback generated code) | no | yes | (generated endpoint) `v0-pp-cli chats restore-message` |

## Transcendence (only possible with our approach)

| Name | Command | Description | WhyItMatters | Buildability |
|------|---------|-------------|--------------|--------------|
| Offline chat & message search | `search "<query>"` | FTS over synced chats + messages; `--json --select` | v0-cli has zero local state; browsing hundreds of chats requires N API calls. One sync, instant grep. | hand-code |
| Sync local mirror | `sync --resources chats,messages,mcp-servers,hooks` | Cursor-paginated pull into SQLite; `--full` deep-syncs messages per chat | Powers search + spend without burning credits re-fetching | spec-emits (generated syncer) |
| Credit spend analytics | `spend --since 7d` | Aggregates usage.tokens + creditsCost from cached messages, grouped by chat/model/day | Every v0 response bills credits; no tool tracks burn. Power users and teams care. | hand-code |
| Model usage breakdown | `spend --by model` | v0-mini/pro/max/max-fast token + cost rollups | Visibility into which model tier costs the team money | hand-code |
| Streaming capture | `chats create --stream` + `messages send --stream` | SSE event stream printed live with parts; `--json` emits event envelope | Async creation is opaque; streams show thinking/file-edits in real time | hand-code (SSE parse) |
| Tail chat activity | `messages tail --chat <id> --follow` | Polls newest messages until finishReason != null; exit code 0 on stop | Watch a long generation to completion without manual polling | hand-code |
| Chat file tree | `chats files --tree` | Local file inventory of a chat (path/encoding), diff-able | Downloading + grepping a zip is the current UX; a tree is instant | hand-code |
| Preview helper | `chats preview --url` | Prints just the preview URL; token for `x-v0-preview-token` header | Embed previews in scripts/CI | spec-emits + hand-code flag polish |
| Doctor | `doctor` | Validates V0_API_KEY + reachability + account scope | Fail-fast auth diagnostics (generated) | spec-emits |

Novel features scored ≥5/10: search, sync, spend, spend-by-model, streaming capture, tail, file tree, preview-helper, doctor.
Hand-code count: 7 (`search`, `spend`, `spend --by model`, `create-stream` SSE wrap, `messages tail`, `files --tree`, `preview --url` flag). Spec-emits: sync + doctor (generated) + 31 absorbed endpoint commands.
