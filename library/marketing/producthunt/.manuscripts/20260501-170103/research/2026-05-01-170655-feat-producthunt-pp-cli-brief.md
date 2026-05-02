# Product Hunt CLI Brief

## API Identity
- **Domain:** Product Hunt (producthunt.com) — daily launch board for new products. Makers post, hunters submit, the community votes and comments. Topic taxonomy organizes launches; collections curate hand-picked sets.
- **Users:** makers/indie founders (launching), hunters (submitting), VCs/scouts (deal flow), tech enthusiasts (discovery), PMs/marketers (competitor and category tracking), researchers/analysts (historical trend analysis).
- **Data profile:** highly relational — Post ↔ Maker/Hunter (User) ↔ Topic ↔ Collection ↔ Comment. Time-series value in tracking rank/upvotes across days. Cursor-paginated everything.
- **Surface choice (this redo):** **GraphQL v2 API as primary tier** (`https://api.producthunt.com/v2/api/graphql`, bearer token from a free personal OAuth app), **public Atom feed at `/feed` as the no-auth fallback tier**. This inverts the prior CLI's primary-vs-fallback choice — see "Status of prior CLI" in User Vision.

## Reachability Risk
- **GraphQL endpoint:** Reachable. Confirmed live with the user's developer token: `viewer`, `posts(first:N)`, `posts(order:VOTES)`, `collections(featured:true)`, `topics(first:N)`, `post(slug:"postiz-3")`, and pagination all work end-to-end.
- **RSS feed:** Reachable (HTTP 200, ~47 entries spanning ~6 weeks).
- **Known data-redaction policy (PH-side, applied identically to both dev-token and OAuth client_credentials):** Confirmed live against the API:
  - `Post.user` (the poster) returns full data (`id`, `username`, `name`, `twitterUsername`)
  - `Post.makers[]` returns `id: "0", username: "[REDACTED]", name: "[REDACTED]"` — **makers are redacted**
  - `Post.comments[].user` returns `id: "0", username: "[REDACTED]", name: "[REDACTED]"` — **commenter identities are redacted**
  - `user(username: "X")` non-self lookup returns `id: "0", username: "[REDACTED]", name: "[REDACTED]", headline: null, twitterUsername: null` — **non-self user lookups are essentially useless** (only `id`-presence + the literal "[REDACTED]" string come back)
  - `viewer` (yourself) returns full data
  - `Collection.user` (curator) is also redacted
  Implication: `users get/posts/voted-posts` for non-self usernames is structurally a stub from the API's side — we ship those commands honestly with output explaining the redaction; we do NOT lean on them as flagship features. Flagship features lean on poster/post/topic/collection data, which IS unredacted.
- **Complexity budget:** Confirmed `X-Rate-Limit-Limit: 6250` / `X-Rate-Limit-Remaining: <N>` / `X-Rate-Limit-Reset: <secs>` returned on every call. Every command that fans out plans for partial-results-with-budget-warning rather than crash. We surface budget in `whoami` and `doctor`.

## Auth Design (revised after live probes)
- **Primary auth: developer token** (`PRODUCT_HUNT_TOKEN`). Single click at the bottom of the OAuth app page; never expires; supports every query the OAuth path supports PLUS `viewer`. This matches `jaipandya/producthunt-mcp-server`'s convention.
- **Alternate auth: OAuth client_credentials** (`PRODUCT_HUNT_CLIENT_ID` + `PRODUCT_HUNT_CLIENT_SECRET`). Useful for CI/automation where users don't want to share a personal token. The CLI exchanges credentials for an access_token, caches it on disk, and refreshes on 401. Has the limitation that `viewer` returns `null` under public scope.
- `auth onboard` defaults to the dev-token flow; `auth onboard --oauth` walks the OAuth path for advanced users. Both surface the callback-URL trick (`https://localhost/callback`) for the OAuth-app form, since both auth types require an OAuth app to be created on PH first.

## Top Workflows
1. **Today's top launches** — `producthunt today` / `producthunt posts list --order=RANKING` for the morning skim. Replaces opening the website each morning.
2. **Deep launch detail** — `producthunt posts get <slug-or-id>` returns name, tagline, votes, comments, makers, topics, media. The `--enrich` flag also pulls comments inline (paginated).
3. **Topic vertical tracking** — `producthunt posts list --topic=artificial-intelligence --posted-after=2026-04-01 --order=VOTES` for VCs/PMs scouting a sector over a window.
4. **Maker / hunter history** — `producthunt users get <username>` plus `producthunt users posts <username>` and `producthunt users voted-posts <username>`. Useful for sourcing and competitive intel even with the redacted `name` field, since `@username` is a stable identity key.
5. **Collection curation lookup** — `producthunt collections list --featured` and `producthunt collections get <slug>`. Editorial collections surface gems that the daily leaderboard misses.
6. **Comment triage** — `producthunt posts comments <slug>` so a maker can export and review hundreds of comments on launch day without OAuth scope friction.
7. **Topic discovery / search** — `producthunt topics search "ai agent"` to find and follow new verticals as they emerge.
8. **No-auth daily skim** — `producthunt feed` (the RSS path) shows the latest ~47 entries for users who have not configured a token yet. Quietly surfaces "configure a token for full data" in its footer.

## Codebase Intelligence
- Source: jaipandya/producthunt-mcp-server (Python, FastMCP). 11 typed MCP tools.
- Auth: `Authorization: Bearer <token>` header. Env var: `PRODUCT_HUNT_TOKEN`.
- Data model: cursor-paginated `posts`, `collections`, `topics`, `comments` (under `post`), `madePosts`, `votedPosts` (under `user`). Standard Relay edges/node shape. Timestamps are ISO 8601.
- Rate limiting: `X-Rate-Limit-Limit / Remaining / Reset` (epoch seconds). 429 responses include `X-Rate-Limit-Reset`. Client must respect these and surface a structured `RATE_LIMIT_EXCEEDED` error with `retry_after`.
- Architecture: single GraphQL endpoint, all reads via POST. Personal developer token bypasses OAuth scope juggling for personal use.

## User Vision
**The user provided unusually rich context up-front.** Captured verbatim in `user-briefing-context.md`. Key points:

- **GraphQL API is the "better" tier.** Free personal API key generated at https://www.producthunt.com/v2/oauth/applications. Onboarding must be friction-free for a developer who has never used PH's OAuth.
- **RSS feed is the "good" fallback.** No-key users still get a useful CLI for the daily skim. Data is more limited (no votes/ranks/comments/makers) and the CLI must surface that honestly.
- **First-class onboarding UX in `--help`, `doctor`, and README:**
  - Direct users to https://www.producthunt.com/v2/oauth/applications
  - Tell them the callback/redirect URL field can be filled with `https://localhost/callback` since it's not used for the personal-token flow
  - Walk them through generating the developer token from the bottom of the OAuth app page
  - Clear `auth set-token` / env var setup
- **Status of prior CLI:** Previously printed on printing-press v1.3.3 (Apr 2026); this is on v3.2.1. Prior brief argued AGAINST GraphQL primacy citing the complexity-pts budget; user explicitly wants the opposite. This redo restructures around GraphQL-primary + RSS-fallback (not GraphQL-as-enrichment-on-top-of-scraping).

## Source Priority
Single-source CLI. Spec source: agentically-composed internal YAML covering both GraphQL and RSS; no Multi-Source Priority Gate needed.

## Data Layer
- **Primary entities:** `post`, `user`, `topic`, `collection`, `comment`. Optionally `feed_entry` (RSS-side, deduped against `post` by post ID).
- **Relations:** post↔user (poster, makers), post↔topic (many-to-many), post↔collection (many-to-many via collection-posts), post↔comment, user↔post (madePosts, votedPosts).
- **Time-series tables:** `post_snapshot(post_id, ts, votes_count, comments_count, day_rank)` for rank-trajectory commands. Only the synced data we capture builds this — PH's API exposes the current state but not historical trajectories.
- **Sync cursor:** per-resource (`posts`, `topics`, `collections`) using `pageInfo.endCursor` plus `posted_after` for incremental refresh.
- **FTS:** `posts_fts` (name + tagline + description + topic_names), `comments_fts` (body for maker triage), `topics_fts` (name + description for "find a topic" search).

## Product Thesis
- **Name:** `producthunt-pp-cli` (Cobra binary). Plugin name: `producthunt`. Display name: `Product Hunt`.
- **Headline:** "Read Product Hunt from your terminal — works token-free for the daily skim, unlocks the full GraphQL API in one onboarding step."
- **Why it should exist:**
  1. Existing CLIs (sunilkumarc/product-hunt-cli, davguij/phcli, sibis/producthunt-cli) are abandoned, scrape-only, or stub commands. Kristories/phunt was archived in Sept 2025.
  2. The leading reach today is `jaipandya/producthunt-mcp-server` (Python MCP). Solid, but Python-only, MCP-only, requires a token before any data appears.
  3. Our CLI: works from minute one without a token (RSS), unlocks the full GraphQL surface with one-step onboarding (callback URL trick), ships SQLite-backed offline search and rank-trajectory commands no other tool offers, and exposes everything as both CLI and MCP.

## Build Priorities
1. **Foundation (P0):** internal YAML spec covering both GraphQL (with auth) and RSS (no-auth). Generated client + store + sync + search + sql.
2. **Absorbed (P1):** match every read-side feature of `jaipandya/producthunt-mcp-server`'s 11 MCP tools as Cobra commands. Posts (get/list/comments), Collections (get/list), Topics (get/list/search), Users (get/posts/voted-posts), Comments (get/list), Viewer (whoami), server-status (doctor extension).
3. **Transcendence (P2):** rank trajectory, topic-trend, momentum/calendar views, no-auth `/feed` daily skim with auto-upgrade hint, `auth onboard` interactive wizard with the callback URL trick baked in, `posts compare` for side-by-side launches, `topic-watch` for offline alerts.
4. **Polish (P3):** flag descriptions referencing the OAuth dashboard URL, doctor with auth-stage-specific guidance ("you have no token; here's how", "you have a token but it's invalid; here's how to regenerate", "you have a valid token; here's your remaining complexity budget").
