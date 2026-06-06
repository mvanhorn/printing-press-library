# X (Twitter) API v2 CLI Brief

## API Identity
- **Domain:** Official programmatic surface for X (formerly Twitter). Source spec: `https://api.twitter.com/2/openapi.json` (== `https://api.x.com/2/openapi.json`), OpenAPI 3.0.0, **X API v2 v2.165**, server `https://api.x.com`, **139 paths / 163 operations**. Security schemes: `BearerToken` (app-only), `OAuth2UserToken` (user-context), `UserToken` (legacy OAuth1).
- **Users:** developers/indie hackers (bots, schedulers, integrations); social-media & growth-marketing managers; researchers/academics/journalists (search + network analysis); brand/monitoring teams; automation operators.
- **Data profile:** Posts/Tweets are the gravity center (author_id→User, conversation_id threads, referenced_tweets reply/quote/retweet edges, attachments→Media, public_metrics). Users, Lists (+members), Spaces, DMs, Bookmarks, Likes, Community Notes, Communities, Trends, News, plus operational Usage (metering) and Compliance (batch jobs).

## Reachability Risk
- **HIGH** — two independent access-reality shifts a naive CLI will trip over:
  1. **Pay-per-use tier overhaul (Feb 2026).** Free tier discontinued for new devs; new default is **credit-based pay-per-use** (≈$0.005/post read, $0.001 read-as-owner, $0.01/post created), hard **2M reads/mo** cap, **no streaming or full-archive search** on PPU. Basic ($200/mo) and Pro ($5k/mo, full-archive + streaming) are legacy/closed-to-new. Enterprise sales-only. Exact credit rates live in the Developer Console, not public pages.
  2. **Programmatic-reply restriction (Feb 23–28 2026).** `POST /2/tweets` now **403s on programmatic replies, cold quote-tweets, and @mentions inside posts** (anti-LLM-spam) across Free/Basic/Pro/PPU; only Enterprise exempt. **Self-reply threads still work** — so `thread compose` survives for self-threads but must detect+warn on mention/quote threads.
- **Error palette the CLI must map to actionable messages:** `403/453` (access level/tier below endpoint), `403 client-not-enrolled` (app not attached to a Project), `403` Feb-2026 reply restriction, `402 Payment Required` (PPU credit/spend exhausted), `429` + monthly cap.
- **App-only Bearer vs OAuth2 user-context:** app-only bearer = read-only public data, **no "me"/writes** (403 on user-context); OAuth2 user-context required for all writes + personal reads. `doctor` must report which token is configured and what it unlocks.
- Evidence: tweepy #1927 (453), twitterdev samples #58 (client-not-enrolled), tweepy #2175 (streaming tier), @XDevelopers reply-restriction announcement, X pricing/rate-limit docs.

## Top Workflows
1. **Search & archive posts to a local store** — `GET /2/tweets/search/recent` (PPU/Basic) or `search/all` (Pro+), paginate `next_token`, persist to SQLite, dedup, offline FTS. Highest value; also most cost-sensitive under PPU.
2. **Pull a user's timeline for analysis** — `GET /2/users/:id/tweets` with `public_metrics,created_at` → engagement/cadence/top-posts.
3. **Monitor mentions** — `GET /2/users/:id/mentions` incremental poll with `since_id`, dedup against store → reputation/brand monitoring.
4. **Compose & post a thread** — markdown → numbered **self-reply** chain via `POST /2/tweets` (`reply.in_reply_to_tweet_id`); draft-first, opt-in to post; warn on Feb-2026 restriction. (Prior novel `thread compose`.)
5. **Manage Lists & bookmarks** — create lists, add/remove members, read list timelines; bookmark/read bookmarks (user-context).

## Data Layer
- **Primary entities:** posts (id, author_id, text, created_at, conversation_id, referenced_tweet ids, lang, public_metrics_json), users (id, username, name, description, public_metrics, pinned_tweet_id), lists (+list_members), bookmarks/likes (per-user join → posts), dm_events/dm_conversations, spaces (snapshot ephemeral state), media (media_key→post), usage + compliance snapshots.
- **Sync cursor:** `since_id`/`newest_id` (canonical incremental key, returned in search `meta.newest_id`); `next_token` for backfill within a window; spaces snapshot per sync; compliance via job IDs.
- **FTS/search:** SQLite FTS5 over post `text`, filterable by author, date range, lang, has:media/has:links, is:reply/is:retweet/is:quote, engagement thresholds — mirroring X search operators. Offline thread reconstruction via `conversation_id`. The local store can offer a **richer** query surface than the live recent-search API — a selling point, and a **cost-control** feature under PPU (read once, query many).

## Codebase Intelligence
- Reference clients: `PLhery/node-twitter-api-v2` (typed, auto-paginator, rate-limit plugin), `tweepy` (defines the error palette), `sferik/x-cli` (the CLI bar to beat).
- Auth header: `Authorization: Bearer <token>`. App-only bearer obtained via `POST /oauth2/token`; user-context via OAuth2 Auth-Code + PKCE.

## User Vision
- **Maximize the MCP / agent surface.** Primary motivation is the large machine delta (4.1.0 → 4.20.1: new MCP surface, auth modes, transport, scoring). Bias the novel-features brainstorm toward richer, better-annotated agent tooling and MCP token efficiency on top of the 163-operation surface — readOnly/destructive/idempotent hints, tool descriptions, named multi-step intents, code orchestration, and remote (stdio+http) transport. Reconcile the two prior novels (`thread compose`, `articles-publish-md`) as keep/reframe/drop with reasons, never silently.

## Auth Decision (for Phase 2 enrichment)
- **Primary credential: app-only Bearer token, env var `X_BEARER_TOKEN`.** Get one at **https://console.x.com/** (create an app, copy its Bearer Token). This is what the operator is actually using; name the variable for the credential.
- Secondary (optional, documented): `X_OAUTH2_USER_TOKEN` for user-context writes/personal reads.
- Propagate the env-var name + console.x.com key URL to config.go, doctor, client, README Auth section, SKILL, and the **MCP server's credential/env description**.

## Product Thesis
- **Name:** X CLI (binary `x-twitter-pp-cli`); brand "X (Twitter)" with "Twitter" retained as a searchable alias. Posts == Tweets (paths still `/2/tweets`).
- **Why it should exist:** The only X CLI that gives humans *and* AI agents an offline-first, searchable local mirror of X — FTS + SQL over archived posts/users/lists without re-spending per-read API credits — plus a 163-operation MCP surface with honest tier/reachability diagnostics. `t`/`x-cli` are live-only with no agent surface; twurl is a raw signer; every existing X MCP server is thin and stateless. Under pay-per-use billing, read-once-query-many is a cost feature, not just convenience.

## Build Priorities
1. **Base regen** of the full 163-operation official surface under the current machine, with **bearer-token auth (`X_BEARER_TOKEN`, console.x.com)** and **MCP Cloudflare pattern** (transport `[stdio, http]`, `orchestration: code`, `endpoint_tools: hidden`) — directly serves the maximize-MCP vision on a >50-tool surface.
2. **MCP intents** layered over the typed endpoints: `search-and-archive`, `user-snapshot`, `conversation-thread`, `monitor-mentions`, `engagement-report`, `compose-thread`, `list-curate`, `publish-article`.
3. **Reconcile prior novels:** keep `thread compose` (self-reply threading; warn on Feb-2026 restriction) and `articles-publish-md` (draft-first browser/GraphQL Draft.js; preserved via regen-merge), reframed under the current machine.
4. **Reachability honesty:** map 453 / client-not-enrolled / 402 / reply-restriction to actionable `doctor` + per-command errors; default reads hit the local store first.
