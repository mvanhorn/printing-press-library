# Reddit CLI Brief

## API Identity
- **Domain**: Social discussion / link aggregator. Reddit's public REST/JSON API at `oauth.reddit.com` (authenticated, full surface) and `old.reddit.com/*.json` (unauthenticated, read-only listings).
- **Users**: Developers automating Reddit workflows, moderators triaging mod queues, researchers archiving threads, content creators tracking posts, power users digesting subreddit signal vs noise.
- **Data profile**: Subreddits, submissions (link/self-posts), comments (deeply nested threads with `MoreComments` continuations), users (Redditors with karma, trophies, history), messages (inbox/sent/modmail), multireddits, wikis, live threads (deprecated but functional), modqueue/reports/spam, flair (link + redditor), subscriptions, saved/upvoted/downvoted listings.

## Reachability Risk
- **Low for the API surface itself**. `oauth.reddit.com` and `old.reddit.com` respond `200` with proper OAuth + `User-Agent`. Reddit enforces 100 QPM (queries per minute) per OAuth `client_id` for free tier (script/installed apps) and a stricter ~10 RPM for unauthenticated calls.
- **Local hazard noted during reachability probe**: this machine's ISP (Biznet Networks) DNS-redirects `www.reddit.com` to a "SafeSurf" content filter. The CLI must default `base_url: https://oauth.reddit.com` (authenticated path) and provide `unauth_base_url: https://old.reddit.com` for public `.json` listings — never `www.reddit.com`. This is a generally-good idea anyway: `old.reddit.com/*.json` is also the API path most third-party tools use because it bypasses the new web app's hydration.
- **Auth required for most surface**. ~85% of endpoints require OAuth2 bearer token. The `.json` suffix path on `old.reddit.com` exposes a useful read-only subset (listings, submission detail, comments, user about, subreddit about) without auth at low rate limits.
- **2023 API pricing change**: Reddit introduced commercial-tier pricing for high-volume third-party apps (which killed Apollo, RIF). The free tier (100 QPM, personal use, scripts, bots) still works fine. This CLI targets personal/automation use — no risk.
- **`User-Agent` is mandatory**. Reddit returns `429` for default Go/Python/curl user agents. The CLI must emit `User-Agent: <product>/<version> by /u/<username>` per Reddit's API rules.

## Top Workflows
1. **Read & triage** — Browse front page, hot/new/top/rising/controversial per subreddit, with `--limit`, `--time`, and `--after` (cursor) for pagination. Get individual submissions + full comment trees (including `MoreComments` expansion). The "I want to skim r/programming for the morning" workflow.
2. **Search across Reddit** — Cross-Reddit and per-subreddit search with `--sort relevance|hot|top|new`, `--time hour|day|week|month|year|all`, `--type link|user|sr`, `--restrict-sr`. The "find every thread that mentions our product launch this week" workflow.
3. **Vote, save, and reply** — Upvote/downvote/unvote, save/unsave, hide/unhide, reply to posts and comments, edit own posts/comments, delete own content. The "I want to interact with Reddit without opening the web UI" workflow.
4. **Submit content** — `submit` (link, self, image, video, crosspost) with subreddit, title, flair, NSFW/spoiler flags, send-replies toggle, OC marking. The "post my blog article to r/programming from a CI script" workflow.
5. **Moderator workflows** — modqueue, reports, spam, edited, modlog, approve/remove/distinguish, ban/unban users, mute/unmute, set-flair (link + user), modmail, removalreason templates, wiki edits. The "I run a subreddit and the dashboard is too slow" workflow.
6. **User analysis** — Get user about, submitted history, comment history, saved/upvoted/downvoted (own only), gilded, controversial, top, trophies, karma breakdown by subreddit. The "is this user a karma farmer or a real contributor" workflow.
7. **Subreddit discovery & analytics** — Popular subreddits, new subreddits, subreddit search, subreddit stats (subscriber count, active users, traffic if mod), default subreddits, recommended (deprecated). The "what subreddits should I check that I'm not subscribed to" workflow.
8. **Inbox & messages** — Inbox listing, unread filter, mentions, replies, sent, mark-read/unread, compose, modmail conversations, modmail reply. The "process my Reddit inbox without the UI" workflow.
9. **Multireddits & subscriptions** — List/get/create/update/delete multireddits, add/remove subreddits, list own subscriptions, subscribe/unsubscribe. The "I keep my reading lists in code" workflow.
10. **Wiki & flair management** — Wiki page get/list/edit/revision-history, flair templates list/create/delete, flair self-assign, link flair set (mod-only). The "wiki + flair is my CMS" workflow.

## Table Stakes (every competitor has at least one of these)
- `get_frontpage_posts` / `get_subreddit_hot|new|top|rising_posts` (every MCP server)
- `get_post_comments` / `get_post_content` / `get_submission_by_url|by_id` (every MCP server)
- `get_user_info` / `get_user_posts` / `get_user_comments` (Arindam, jordanburke)
- `get_subreddit_info` / `get_trending_subreddits` (every MCP server)
- `search_reddit` / `search_posts` (Arindam, jordanburke)
- `create_post` / `reply_to_post` / `edit_post` / `edit_comment` / `delete_post` / `delete_comment` (jordanburke)
- OAuth2 (script app flow, refresh token flow)
- Rate limit handling, automatic retry
- Custom User-Agent enforcement
- Pagination via `after`/`before` cursor

## Data Layer
- **Primary entities**: `subreddits`, `submissions`, `comments`, `redditors` (users), `messages`, `multireddits`, `subscriptions`, `mod_actions` (modlog), `saved`, `upvoted`, `downvoted`, `inbox`.
- **Sync cursor**: Reddit listings use `after`/`before` opaque cursors (e.g., `t3_abc123` for submissions). Store last seen `created_utc` per `(entity, key)` for incremental sync.
- **FTS/search**: FTS5 indexes on `submissions(title, selftext)`, `comments(body)`, `subreddits(public_description, description)`, `redditors(subreddit.public_description)`. This is where transcendence happens — Reddit's native search is famously bad; local FTS5 over your own saved/sync'd content is gold.
- **Listings to persist**: own subscriptions, multireddits, saved/upvoted/downvoted (current user), submitted history (per user), comment history (per user), modqueue snapshots (with timestamps to compute "time to triage"), modlog (mod-only).
- **Computed views**: karma trends per subreddit, comment velocity per submission, "stale" mod-queue items, ghost-followers / inactive subscribers (not measurable directly, skip), best-time-to-post per subreddit (computed from submitted history vs scores).

## Codebase Intelligence

Source: indexed READMEs from competing MCP servers (Arindam200, jordanburke, Hawstein, adhikasp) and PRAW docs.

- **Auth**:
  - Script app: `client_id` + `client_secret` + `username` + `password` → POST `https://www.reddit.com/api/v1/access_token` with HTTP Basic auth → bearer token (1 hour TTL).
  - Web/installed app: OAuth code/implicit grant flow with `state`, `redirect_uri`, `duration=permanent` for refresh token.
  - Application-only OAuth: client credentials grant for read-only access without user.
  - Refresh token flow: POST `/api/v1/access_token` with `grant_type=refresh_token`.
  - All API calls: `Authorization: Bearer <token>` + `User-Agent: <product>/<version> by /u/<username>`.
- **Data model**: Thing IDs prefixed by type — `t1_` (Comment), `t2_` (Account/Redditor), `t3_` (Submission/Link), `t4_` (Message), `t5_` (Subreddit), `t6_` (Award/deprecated). Listings wrap `kind: "Listing"` with `data.children[].data` payload. Comments come as nested `replies` (sometimes wrapped in `MoreComments` with `children: []` of IDs that need a second `morechildren` call to expand).
- **Rate limiting**: Reddit returns `X-Ratelimit-Used`, `X-Ratelimit-Remaining`, `X-Ratelimit-Reset` headers. Free tier: 100 QPM per OAuth client_id; non-OAuth: ~10 RPM. On `429`, sleep `Reset` seconds (typically 60-600).
- **Architecture key insight**: The `.json` suffix on any Reddit URL returns JSON; this is how `old.reddit.com` exposes a read-only API. The `oauth.reddit.com` host is the same surface but accepts `Authorization: Bearer` and writes. The CLI should support both: `--no-auth` flag for cheap read-only ops via `old.reddit.com`, default OAuth via `oauth.reddit.com`.
- **MoreComments expansion**: A submission's comment tree may include `MoreComments` placeholder objects with a list of IDs. Fully expanding requires recursive POST to `/api/morechildren` (32 IDs max per call). PRAW does this transparently; the CLI should expose `--depth N`, `--threshold N` (skip MoreComments below score), and `--expand-all` flags.

## User Vision
User selected "Let's go (recommended)" — no specific vision provided. Standard build: match every MCP and SDK feature, then transcend with offline FTS5 + mod-queue triage helpers + cross-subreddit signal aggregation.

## Product Thesis
- **Name**: `reddit-pp-cli` — Reddit CLI for the terminal that absorbs every feature from PRAW, snoowrap, RedditWarp, and every Reddit MCP server, and adds an offline-first SQLite layer, cross-subreddit aggregation, and mod-workflow primitives nothing else has.
- **Why it should exist**:
  - Every existing Reddit CLI is dead (rtv, tuir) or trivial (one-off post browsers).
  - The MCP servers are great for agent integration but expose 8-15 tools, not the full 100+ endpoint API surface.
  - PRAW is the gold standard but it's a library, not a CLI — you still write Python.
  - No tool offers offline FTS5 over your own Reddit history (saved / submitted / upvoted) — Reddit's own search has been an industry-standard joke for a decade.
  - No tool offers mod-queue triage primitives like "what's been sitting in modqueue for >24h" or "which reports have the highest reporter:remove ratio in the last week."
  - No tool offers cross-subreddit signal aggregation like "every comment by /u/X across the subs I moderate, with mod-action history."

## Build Priorities
1. **Foundation** (Phase 3 P0): OAuth2 (script + refresh-token flows), config (env vars + `~/.config/reddit-pp-cli/config.toml`), `doctor`, SQLite store schema for all 12 primary entities, `sync` (incremental cursor-based per entity), FTS5 search.
2. **Absorb — full surface** (Phase 3 P1): Every endpoint catalogued by PRAW (read + write + mod), expressed as typed Cobra commands. This is the ~140-endpoint endpoint surface — not "top 5."
3. **Transcend** (Phase 3 P2): The novel features below.
4. **Polish**: README, agent context, MCP exposure annotations, examples.

## Phase 1.6 Pre-Browser-Sniff Auth Intelligence
- **Auth profile**: API key (`client_id` + `client_secret`) for app authentication, plus a per-user credential (script app uses `username` + `password`; web/installed apps use OAuth refresh token). Captured in `REDDIT_CLIENT_ID`, `REDDIT_CLIENT_SECRET`, `REDDIT_USERNAME`, `REDDIT_PASSWORD`, optionally `REDDIT_REFRESH_TOKEN`, and `REDDIT_USER_AGENT`.
- **User already opted in** to providing credentials during the API Key Gate. No further prompt needed.

## Browser-Sniff Gate Pre-Decision
- Reddit publishes complete docs at https://www.reddit.com/dev/api (the OAuth2 wiki is archived but accurate). PRAW (https://praw.readthedocs.io) covers the entire current endpoint surface — there is no gap competitors are exploiting that PRAW doesn't cover.
- **Decision matrix says**: spec found (via PRAW's comprehensive code overview that maps to every endpoint), no gaps. **→ Phase 1.7 should skip silently**.

## Source Priority
Single source: Reddit (`oauth.reddit.com` + `old.reddit.com` fallback for unauth reads). No combo / multi-source.
