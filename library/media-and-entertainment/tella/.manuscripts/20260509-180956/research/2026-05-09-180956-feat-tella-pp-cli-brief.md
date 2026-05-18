# Tella CLI Brief

## API Identity
- **Domain:** Tella is a creator-focused screen + face recording platform. Users record videos that compose into clips, organize them into playlists, and share/embed/export. The Public API exposes the editing primitives the web app uses: clip-level effects (blur, highlight, zoom, layout), transcript retrieval, silence/waveform analysis, exports, and webhooks.
- **Surfaces:** single OpenAPI 3.0.3 spec (`https://www.tella.com/docs/openapi.json`, 51 operations on 36 paths), separate webhooks spec (`https://www.tella.com/docs/webhooks.json`).
- **Base URL:** `https://api.tella.com/v1/...`
- **Auth:** Bearer token from account settings; `Authorization: Bearer <key>`. Single auth scheme across all endpoints.
- **Users:** creator-economy folks doing async-video updates, sales SDRs sending personalized walkthroughs, support teams recording bug repros, founders shipping product demos.

## Reachability Risk
**Low.** Standard SaaS REST API. No bot detection, no rate-limit issues at typical usage. One known consideration: the official MCP server already exists and Tella may add features there first, but the Public API is documented and stable (v1.0.0).

## Top Workflows
1. **List/inspect videos** — `videos list`, `videos get`, `clips list`
2. **Manage playlists** — `playlists list/get/create/update/delete`, add/remove video
3. **Retrieve transcripts** — cut transcript (post-edit) or uncut (raw); the highest-value read endpoint
4. **Apply clip-level edits** — blurs, highlights, zooms, layouts, cuts, filler-removal, reorder
5. **Export videos** — kick off an export, retrieve thumbnails, fetch waveforms
6. **Webhook subscriptions** — register endpoint, retrieve signing secret, replay messages from inbox

## Table Stakes
Every Public API operation must be reachable:
- Playlists: list, get, create, update, delete; add/remove video
- Videos: list, get, update, delete; collaborator add; duplicate; export; thumbnail
- Clips: list, get, update, delete; cut; duplicate; reorder; silences; sources; waveform; thumbnail; transcripts (cut + uncut)
- Clip effects (each: list, add, update, remove): blurs, highlights, layouts, zooms; plus remove-fillers
- Webhooks: create endpoint, delete endpoint, get signing secret; list and get messages

## Data Layer
- **Primary entities:** `videos`, `clips`, `playlists`, `clip_effects` (blurs/highlights/zooms/layouts), `transcripts`, `webhook_messages`
- **Sync cursors:** standard pagination on list endpoints; `created_at` / `updated_at` for incremental
- **FTS5 winners:** `transcripts(text)` (cross-video transcript search is the killer agent feature); `videos(title, description)`
- **Why local-first matters:** an agent searching across a workspace's transcripts ("which video mentions our pricing change?") is impossible against the stateless Public API alone — you'd have to fetch every transcript on every query.

## Codebase Intelligence
- Source: Tella's own llms.txt index + the official OpenAPI spec
- Auth: Bearer token via `Authorization: Bearer`
- Webhooks: 5 events (`Video created`, `Video viewed`, `View milestone reached`, `Export ready`, `Transcript ready`); HMAC signing via per-endpoint secret
- Architecture: composition pattern — playlists contain videos, videos contain clips, clips contain effects + sources. Clip-level edits are first-class API operations (not just metadata writes).

## User Vision
User pitched Tella as a category-open candidate (no async-video CLIs in the public library). Used phrasing "what hasn't been made into a CLI." Implies emphasis on differentiation vs the existing MCP server and bulk/agent ergonomics.

## Product Thesis
- **Name:** `tella-pp-cli` (binary), API slug `tella`
- **Why it should exist when the official MCP exists:**
  1. **Human + agent in one binary.** The MCP is agent-only; this is also CLI-callable, scriptable, pipeable.
  2. **Local SQLite store + transcript FTS.** Search across a workspace's transcripts offline, in milliseconds. Impossible against the stateless API or the stateless MCP.
  3. **Bulk operations.** Apply silence-removal, exports, or filler-removal across a playlist or filter expression in one command.
  4. **Webhook capture-and-replay locally.** `webhooks tail` watches the API's webhook-message inbox and replays them to a local handler — usable for development without ngrok.
  5. **Single-binary install.** `brew install`-style install instead of MCP config.
- **Differentiation vs the official Tella MCP:**
  1. Local-first transcript search; MCP re-fetches every call.
  2. Bulk + composition operations agents would otherwise have to chain manually.
  3. Typed exit codes, `--json --select`, dry-run on mutations — first-class agent contract.

## Build Priorities
1. **P0 — Auth + HTTP client.** Bearer token via env (`TELLA_API_KEY`); doctor checks reachability + token validity.
2. **P0 — SQLite store + sync.** Tables for videos, clips, playlists, transcripts, clip_effects, webhook_messages. FTS5 on `transcripts.text` and `videos.title|description`.
3. **P1 — Absorbed parity.** All 51 OpenAPI operations as Cobra subcommands.
4. **P2 — Transcendence.** Cross-video transcript search, watch-milestones digest, bulk silence/filler trim, exports waitlist, webhook tail/replay, workspace stats, compare-clips diff.

## Sources
- [Tella OpenAPI spec](https://www.tella.com/docs/openapi.json) — 51 operations
- [Tella docs index](https://tella.com/docs/llms.txt)
- [Tella API reference](https://www.tella.tv/help/integrations/public-api-and-webhooks)
- [Tella MCP server (official)](https://www.tella.tv/help/integrations/mcp-server)
- [API tracker entry](https://apitracker.io/a/tella-tv)
