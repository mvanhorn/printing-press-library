# Product Hunt CLI — Absorb Manifest (persona-validated)

## Source tools surveyed

| Tool | Form | Status | Coverage source |
|------|------|--------|-----------------|
| jaipandya/producthunt-mcp-server | Python MCP, 11 typed tools | Active, well-maintained | Source code (queries.py, client.py, tools/*) |
| @yilin-jing/producthunt-mcp | TypeScript MCP | Active, smaller | npm description |
| producthunt (npm) | v1 SDK | Outdated (REST) | npm README |
| product-hunt (npm) | unofficial wrapper | Outdated | npm README |
| sunilkumarc/product-hunt-cli | Node CLI | 21★, scrapes website | README |
| Kristories/phunt | Node CLI | Archived Sept 2025 | README |
| davguij/phcli | oclif/TypeScript | Stub-only, abandoned | README (only `hello`/`help` shipped) |
| sibis/producthunt-cli | Node CLI | Trending-only | README |
| Mayandev/hacker-feeds-cli | Multi-source CLI | Active | README (`product -c -p`) |
| producthunt/producthunt-api | Official OAuth starter | Active | README, schema.graphql |

The bar to beat: **jaipandya/producthunt-mcp-server's 11 typed MCP tools.** Every other CLI is a strict subset and abandoned, scrape-only, or stub.

## Personas (user-named)

- **Persona A — Indie founder launching this week.** Refreshes the leaderboard on launch day; needs to triage the comment flood for real questions; wants to know how their launch ranks vs the day's cohort; wants benchmarks ("what does a 'good' launch look like at hour 6 in this category").
- **Persona B — Marketer / competitive research.** Weekly category sweeps; monthly trend reports; on-demand competitor deep-dives; wants slide-deck-ready snapshots, brand-mention search, look-alike competitor finder, calendar of when launches happen.

Full persona-vs-feature scoring matrix is in `research/persona-validation.md`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get post details by id/slug | jaipandya MCP `get_post_details` | `producthunt posts get <id\|slug>` with `--include-comments`, `--include-makers`, `--json`, `--select` | Offline if synced; `--select` field projection; FTS-backed slug fallback; per-user redaction note in output |
| 2 | List/filter posts | jaipandya MCP `get_posts` | `producthunt posts list --topic --order --count --featured --posted-before --posted-after` | Local-first via store, falls through to GraphQL on cache miss; `--csv`, `--select`, typed exit codes |
| 3 | Get individual comment | jaipandya MCP `get_comment` | `producthunt comments get <id>` | Same surface; agent-native `--json` |
| 4 | List comments on a post | jaipandya MCP `get_post_comments` | `producthunt posts comments <id\|slug> --order --count --after` | Stores comments locally for FTS digest |
| 5 | Get collection by id/slug | jaipandya MCP `get_collection` | `producthunt collections get <id\|slug>` | Includes paginated post-list inline; `--select` projection |
| 6 | List collections with filters | jaipandya MCP `get_collections` | `producthunt collections list --featured --user-id --post-id --order --count` | Local-first; `--csv` |
| 7 | Get topic by id/slug | jaipandya MCP `get_topic` | `producthunt topics get <id\|slug>` | Returns followers/posts counts; offline once synced |
| 8 | Search topics by query | jaipandya MCP `search_topics` | `producthunt topics search <query> --followed-by-user-id --order --count` | Falls through to local FTS when offline |
| 9 | Get user profile | jaipandya MCP `get_user` | `producthunt users get <id\|username>` | **Honest about redaction.** PH redacts non-self `user()` lookups to `id:"0", username:"[REDACTED]"`. Command ships with explicit "this returns redacted data — only `whoami`/`viewer` returns real user data" callout in `--help` and the response `_meta` block. Kept for jaipandya MCP parity but honestly limited. |
| 10 | User's made posts | jaipandya queries `USER_POSTS_QUERY` | `producthunt users posts <username>` | Same redaction caveat surfaced; **post data IS unredacted** so this is still useful for understanding a user's launch history (you just don't get their human name) |
| 11 | User's voted posts | jaipandya queries `USER_VOTED_POSTS_QUERY` | `producthunt users voted-posts <username>` | Same redaction caveat |
| 12 | Authenticated viewer | jaipandya MCP `get_viewer` | `producthunt whoami` (also `producthunt auth whoami`) | Bonus: shows complexity-budget remaining, surfaces auth mode |
| 13 | Server/API health check | jaipandya MCP `check_server_status` | `producthunt doctor` (extended) | Auth-stage-aware: "no token", "invalid", "valid + N budget remaining" |
| 14 | Trending today / leaderboard | sunilkumarc home, Kristories `posts`, Mayandev `product -c` | `producthunt today` (alias for `posts list --order=RANKING --posted-after=midnight`) | One-keystroke daily skim |
| 15 | Latest posts | sunilkumarc latest, Kristories `posts new` | `producthunt recent` (alias for `posts list --order=NEWEST`) | Same |
| 16 | Posts by category/topic | sunilkumarc, Kristories | Covered by `posts list --topic=<slug>` | Multi-topic AND/OR via repeat-flag |
| 17 | Search posts by name | sunilkumarc product search | `producthunt search <query>` (local FTS over synced posts) | True full-text via SQLite FTS5 |
| 18 | Find products by author | sunilkumarc author search | `producthunt users posts <username>` | Same surface as users-posts |
| 19 | Public no-auth daily feed | hacker-feeds-cli `product -c` | `producthunt feed --count N --past N` | RSS-backed; auto-upgrade hint to GraphQL tier |
| 20 | "Me" view | Kristories `me` | `producthunt whoami` | Uses GraphQL `viewer` |
| 21 | "My posts" / "My products" | Kristories `me posts` / `me products` | `producthunt users posts $(whoami)` (computed) | Convenience wrapper if requested |
| 22 | Sync to local store | Generator default | `producthunt sync --resource posts\|topics\|collections --posted-after <date>` | Cursor-based; resumable |
| 23 | SQL query against local store | Generator default | `producthunt sql "<query>"` | Read-only; FTS5-backed |
| 24 | Help / version / completion | All CLIs | Cobra default | Standard |

**Stubs:** none. All 24 absorbed features ship as full implementations. Items 14-21 are aliases/wrappers over (2)/(9)/(10)/(12) and add zero new GraphQL surface.

## Transcendence (persona-validated against indie-founder + marketer)

See `research/persona-validation.md` for the full re-scoring. Original 11 features were re-scored against the two named personas; 2 dropped, 1 reshaped, 4 new persona-driven features added, 5 retained as-is.

### Founder-launch-day cluster (5 commands, all 8/10+)

| # | Feature | Command | Persona ritual it serves | Score |
|---|---------|---------|-------------------------|-------|
| F1 | Launch-day tracker | `producthunt posts launch-day <my-slug>` | "I keep refreshing the leaderboard." Renders YOUR launch's trajectory side-by-side with today's top 5 (sync-driven). | 9/10 |
| F2 | Hour-by-hour benchmark | `producthunt posts benchmark --topic <slug> --hour 6` | "What does a 'good' launch look like at hour 6 in my category?" Reports percentile curves (top-10 / top-50) for the topic from accumulated local history. Requires a local store of past launches. | 9/10 |
| F3 | Trajectory (foundation) | `producthunt posts trajectory <slug>` | Plots a single launch's votes-over-time. Foundational for F1; also useful standalone for retro analysis. PH's GraphQL is point-in-time; trajectory only exists in the local store. | 9/10 |
| F4 | Comments → questions | `producthunt posts questions <slug>` | "I miss real customer questions in the comment flood." Surfaces only comments that look like genuine questions (regex `\?` + heuristic verbs: "how", "what's", "can it", "does it"); ranks by vote. | 8/10 |
| F5 | Side-by-side compare | `producthunt posts compare <slug1> <slug2> [<slug3>...]` | Column-aligned comparison of N launches: votes, comments, topics, tagline, url, launch-time delta. Founder benchmarking + marketer competitive set. | 8/10 |

### Marketer-research cluster (4 commands, all 8/10+)

| # | Feature | Command | Persona ritual it serves | Score |
|---|---------|---------|-------------------------|-------|
| M1 | Category snapshot | `producthunt category snapshot --topic <slug> --window weekly\|monthly` | "Build a slide-deck-ready brief of category state." Single-output: leaderboard for the window + momentum delta vs prior window + most active poster handles + top emerging tags from taglines. Subsumes the original "topics momentum" idea. | 9/10 |
| M2 | Brand-mention grep | `producthunt posts grep --term "claude" --since 7d --topic <slug>` | "Find any launch in the window with my brand or competitor's brand mentioned in tagline/description." Local FTS5 over the store. | 8/10 |
| M3 | Lookalike launches | `producthunt posts lookalike <slug>` | "Find prior launches in this topic that look similar — by topic overlap + tagline FTS rank." Competitive-set discovery. | 8/10 |
| M4 | Launches calendar | `producthunt launches calendar --topic <slug> --week WNN` | "Pick a launch slot." Shows what launched what day this week (and prior weeks for context), with hour-of-day distribution. | 8/10 |

### Cross-persona / monitoring (1 command)

| # | Feature | Command | Why | Score |
|---|---------|---------|-----|-------|
| X1 | Topic watch (offline diff) | `producthunt topics watch <slug> --min-votes 200` | Detects new posts crossing a vote threshold since the last sync. PH has no webhooks; we synthesize via local diff. Useful for marketer scheduled jobs. | 7/10 |

### Agent-native (2 commands)

| # | Feature | Command | Why | Score |
|---|---------|---------|-----|-------|
| A1 | Time-window posts | `producthunt posts since <duration>` | Local-first; falls through to live `posts(postedAfter:)` for the gap. Agent UX. | 7/10 |
| A2 | Agent context snapshot | `producthunt context --topic <slug> --since 24h --json` | Single JSON blob: top posts + top comments + topic state + viewer. One call replaces four. | 7/10 |

### Folded enhancements (no new commands; quality lift on generator-emitted commands)

- `producthunt feed` — token-free RSS tier with auto-upgrade hint footer
- `producthunt whoami` — reports `X-Rate-Limit-Remaining` budget + auth mode (dev-token vs oauth-client)
- `producthunt auth onboard` — interactive wizard, dev-token by default, `--oauth` alternate, callback-URL trick baked in
- `producthunt doctor` — auth-stage-aware diagnostic with actionable next-step per stage

### DROPPED from prior draft (with reasons)

- `collections outbound-diff` — "what can SQLite do" feature; neither named persona requested editorial scouting.
- `comments-digest` (generic FTS) — replaced by sharper `posts questions` aimed at the actual launch-day-triage ritual.
- `topics momentum` — subsumed by `category snapshot`, which delivers the same data plus a slide-deck-ready frame.

## Combined inventory

- **Absorbed:** 24 features (all from competing tools, matched + agent-native)
- **Transcendence (standalone):** 12 commands (5 founder-launch-day + 4 marketer-research + 1 monitoring + 2 agent-native), all persona-validated, none below 7/10
- **Folded enhancements:** 4 (feed, whoami, auth onboard, doctor)
- **Total user-facing commands:** ~30 (some absorbed are flag-variants of the same root)
- **Stubs:** 0

## Reference: GraphQL queries we'll wrap

Pulled verbatim from `jaipandya/producthunt-mcp-server/src/product_hunt_mcp/api/queries.py`:

- `POST_QUERY` (single post by id|slug, includes user/topics/media/makers)
- `POSTS_QUERY` (paginated posts with filters: order, topic, featured, url, postedBefore, postedAfter)
- `COMMENT_QUERY` (single comment by id)
- `COMMENTS_QUERY` (post.comments paginated)
- `COLLECTION_QUERY` (single collection with posts inline)
- `COLLECTIONS_QUERY` (paginated collections with filters)
- `TOPIC_QUERY` (single topic by id|slug)
- `TOPICS_QUERY` (paginated topics with search filter)
- `USER_QUERY` (user by id|username)
- `USER_POSTS_QUERY` (user.madePosts paginated)
- `USER_VOTED_POSTS_QUERY` (user.votedPosts paginated)
- `VIEWER_QUERY` (authenticated user's data + counts)

Auth: `Authorization: Bearer <token>`. Rate-limit headers: `X-Rate-Limit-Limit/Remaining/Reset` (epoch). 429 includes `X-Rate-Limit-Reset`.

## Honest constraints (must surface in CLI + README)

1. **Targeted PH redaction (confirmed live).** Applies to BOTH dev-token and OAuth client_credentials — it is a global PH policy, not a token-permission. Affected:
   - `Post.makers[]` → `id:"0", username:"[REDACTED]", name:"[REDACTED]"`
   - `Post.comments[].user` → same
   - `user(username:)` non-self lookup → entire user record returns redacted
   - `Collection.user` (curator) → redacted
   Unaffected: `Post.user` (the poster), `Topic.*`, `Collection.{name,description,tagline,followersCount,posts}`, `viewer` (yourself).
2. **Complexity-points budget (~6,250 / 15 min per token, confirmed via `X-Rate-Limit-*` headers).** `whoami` reports remaining budget; commands that fan out warn and offer `--budget-aware` early-exit.
3. **RSS feed is intentionally limited.** No votes, ranks, comments, makers, full descriptions. The `feed` command makes this explicit and offers `auth onboard` as the upgrade path.
4. **`viewer`/`whoami` requires the dev-token auth mode.** Under OAuth client_credentials (public scope), `viewer` returns `null`. The `whoami` command surfaces this with an actionable hint to switch modes.
