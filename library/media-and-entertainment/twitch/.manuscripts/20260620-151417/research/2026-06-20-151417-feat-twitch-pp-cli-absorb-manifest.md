# Twitch CLI — Absorb / Transcend Manifest

Run ID: 20260620-151417

## Absorb layer (API surface)

The CLI mirrors the Twitch Helix API across 30 resource groups / 144 endpoints,
including: `games` (and `games get-top`), `streams`, `channels`, `clips`,
`videos`, `users`, `chat` (chatters, emotes, badges, settings), `channel-points`
(custom rewards + redemptions), `polls`, `predictions`, `subscriptions`,
`moderation`, `bits`, `charity`, `goals`, `hypetrain`, `schedule`, `teams`,
`whispers`, `eventsub`, `twitch-helix-analytics`, and `twitch-helix-search`
(channels + categories). The `analytics` and `search` Helix resources are
prefixed `twitch-helix-*` because their bare names collide with the framework's
local transcend commands.

Auth is OAuth2 `client_credentials` via `auth login`, which mints and persists
an app access token. Every request carries both the bearer token and the
required `Client-Id` header (hand-patched into the client).

## Transcend layer (novel features beyond the raw API)

| Feature | Command | What it adds |
|---------|---------|--------------|
| Local SQLite Sync | `sync` | Mirrors Helix data (top games, streams, global emotes, content labels, EventSub subs) into a local SQLite store for offline use |
| Full-Text Search | `search` | Full-text search over synced records, with a live category lookup fallback when the store is empty |
| Local Analytics | `analytics` | count / group-by / summary queries over synced data without re-hitting the rate-limited API |
| Compound Workflows | `workflow` | chains multiple Helix operations into one agent-friendly invocation |

These four commands are absent from every existing Twitch tool surveyed — the
official `twitchdev/twitch-cli` (dev scaffolding, no persistence) and the
library/bot ecosystem (tmi.js, TwitchLib, TwitchIO, twurple, nicklaw5/helix,
twitch_api), all of which are stateless clients that treat Twitch as the system
of record. They are the CLI's differentiation and are recorded as
`novel_features` in `.printing-press.json`.

### Verified live

The transcend pipeline was verified end-to-end against the live API with an app
access token: `sync` (349 records across the 5 default resources, 0 warnings),
`search fortnite` (matched synced stream data), and `analytics --type streams
--group-by game_name` (real aggregation: Just Chatting 4, Fortnite 2,
Overwatch 2, ...). `games get-top` and `streams get` returned live data
(`source: live`).

## Disclosed gaps

- **User-scope sync resources**: the default sync set is the app-token-viable
  subset (games-top, streams, content-classification-labels, chat-emotes-global,
  eventsub-subscriptions). Resources needing a user OAuth token or a required
  `broadcaster_id`/`user_id` parameter (clips, videos, users, followers,
  analytics reports) stay reachable via `--resources` but are not synced by
  default.
- **Search live fallback**: `search` resolves against the local corpus; with an
  empty store it falls back to a live category lookup rather than failing, so a
  prior `sync` is needed for rich cross-resource search.

## MCP

Twitch's large surface (144 endpoints > 50 threshold) triggers the code
orchestration MCP pattern: endpoint tools are hidden behind orchestration with
stdio + http transports, keeping the agent-facing tool count manageable.
