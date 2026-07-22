# Twitch CLI — Research Brief

Run ID: 20260620-151417
API: Twitch Helix API
Spec: community OpenAPI 3.0, vendored locally as `spec.json`
Category: media-and-entertainment

## Source

Twitch does not publish a first-party OpenAPI document. The community project
[DmitryScaletta/twitch-api-swagger](https://github.com/DmitryScaletta/twitch-api-swagger)
auto-generates an OpenAPI spec from the official Twitch API reference
(`https://dev.twitch.tv/docs/api/reference`) with manual fixes. The vendored
`spec.json` is derived from that document (95 paths -> 30 resource groups, 144
endpoints) and covers the full Helix surface: games, streams, channels, clips,
videos, users, chat, channel points, polls, predictions, subscriptions,
moderation, EventSub, analytics, and search.

## Spec corrections (vendored)

Two corrections were applied before generation:

1. **Auth flow.** The community spec only declared the OAuth2 `implicit`
   browser flow. This CLI targets the `client_credentials` grant (app access
   token), which is Twitch's recommended path for server-to-server access to
   public data and works without a user-consent browser round trip. The
   security scheme's flow was rewritten to `clientCredentials` with the real
   token endpoint `https://id.twitch.tv/oauth2/token`.
2. **Text hygiene.** The upstream descriptions contained Unicode smart
   punctuation (curly quotes, en/em dashes) that rendered as mojibake in
   generated help text. These were normalized to ASCII equivalents.

The server URL `https://api.twitch.tv/helix` was already correct.

## Auth

OAuth2 `client_credentials` (app access token). Register a free application at
`https://dev.twitch.tv/console/apps` to obtain a Client ID and Client Secret,
set `TWITCH_CLIENT_ID` and `TWITCH_CLIENT_SECRET`, and run `auth login` to mint
and persist a bearer token (the generated client auto-mints and caches it).

Twitch Helix requires a `Client-Id` header on **every** request in addition to
the bearer token, and the ID must match the client the token was minted for.
The generated client was hand-patched to emit this header (recorded in
`.printing-press-patches/twitch-client-id-header.json`); without it every call
returns `401 Client-Id header required`.

An app access token can read all public data that does not belong to a specific
authenticated user — top games, live streams, global emotes, content
classification labels, EventSub subscriptions, plus any resource queried by an
explicit id/login (games by id, videos by id, channel info by broadcaster_id).
Resources scoped to a logged-in user (bits, subscriptions, moderation,
follower management, analytics reports) require a user OAuth token and stay
reachable via flags but are not part of the default surface.

## Default sync surface (vendored)

`defaultSyncResources` was trimmed (recorded in
`.printing-press-patches/twitch-default-sync-app-token-viable.json`) to the
resources an app token can fetch with no required parameter: `games-top`,
`streams`, `content-classification-labels`, `chat-emotes-global`,
`eventsub-subscriptions`. Out of the box `sync` returns ~349 records with zero
access-policy warnings. The remaining resources stay reachable via
`--resources` for users who supply a user token or required ids.

## Competitive landscape

The Twitch ecosystem is dominated by stateless API client libraries and chat/
bot frameworks, not terminal-native CLIs with a local data layer:

- twitchdev/twitch-cli — official Go CLI, ~677 stars. Purpose is local dev
  scaffolding (mock EventSub server, event triggering, an authenticated
  passthrough `api` command); it persists nothing and queries no local store.
- tmijs/tmi.js — JS chat (IRC) library, ~1.6k stars (unmaintained since 2021).
- TwitchLib/TwitchLib — C# chat/API/EventSub library, ~884 stars.
- TwitchIO/TwitchIO — async Python API + bot framework, ~881 stars.
- twurple/twurple — modular TypeScript library suite, ~741 stars.
- nicklaw5/helix — Go Helix REST client, ~273 stars (stateless wrapper).
- twitch-rs/twitch_api — Rust Helix/EventSub library, ~183 stars.

None offer a local SQLite mirror, full-text search over synced Twitch data,
offline analytics/aggregation, or an MCP server. Every one treats Twitch as the
system of record and holds no durable local state. That statefulness gap is the
basis for this CLI's transcend layer.

## Recommendation

Proceed. Clean reachable REST surface (probe: standard HTTP, no bot
protection), free self-serve app registration, an auth model consistent with
existing library OAuth entries, a live-verified `client_credentials` flow, and
a clear novelty story (local sync + search + analytics + MCP) over the existing
stateless wrappers and the dev-focused official CLI.
