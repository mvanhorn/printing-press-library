# Product Hunt CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List posts (today/featured/newest) | jaipandya/producthunt-mcp-server `get_posts` | `posts list [--featured] [--sort RANKING\|NEWEST\|VOTES]` | Offline after sync, `--json`, `--select`, `--limit` |
| 2 | Filter posts by topic | jaipandya/producthunt-mcp-server `get_posts(topic=)` | `posts list --topic <slug>` | Works offline after sync, FTS on topic name |
| 3 | Filter posts by date range | jaipandya/producthunt-mcp-server `get_posts(posted_before/after=)` | `posts list --after <date> --before <date>` | ISO date args, composable with other filters |
| 4 | Sort posts (RANKING/NEWEST/VOTES/FEATURED_AT) | jaipandya/producthunt-mcp-server | `posts list --sort <order>` | Typed enum, tab-completable |
| 5 | Get post details (name, tagline, votes, makers) | jaipandya/producthunt-mcp-server `get_post_details` | `posts get <slug\|id> [--json]` | Maker list, media, rating fields, `--select` for agent |
| 6 | Get post comments | jaipandya/producthunt-mcp-server `get_post_comments` | `posts comments <slug> [--limit] [--sort VOTES\|NEWEST]` | Sorted by votes, threaded view |
| 7 | Get single comment | jaipandya/producthunt-mcp-server `get_comment` | `comments get <id>` | Shows parent + replies chain |
| 8 | Get collection details | jaipandya/producthunt-mcp-server `get_collection` | `collections get <slug\|id>` | With posts list |
| 9 | List collections | jaipandya/producthunt-mcp-server `get_collections` | `collections list [--featured] [--user <id>]` | Offline cache |
| 10 | Get topic details | jaipandya/producthunt-mcp-server `get_topic` | `topics get <slug\|id>` | Follower count, post count, isFollowing |
| 11 | Search topics | jaipandya/producthunt-mcp-server `search_topics` | `topics search <query>` + offline FTS | Works offline after sync |
| 12 | Get user profile | jaipandya/producthunt-mcp-server `get_user` | `users get <username\|id>` | Made products list, follower/following counts |
| 13 | Get authenticated viewer | jaipandya/producthunt-mcp-server `get_viewer` | `users me` | Shows auth status, profile, recent activity |
| 14 | Health check | jaipandya/producthunt-mcp-server `check_server_status` | `doctor` (generated) | Token validation, API connectivity, store stats |
| 15 | Browse trending products | sunilkumarc/product-hunt-cli home | `posts today` (alias for `posts list --featured --sort RANKING`) | Offline after sync |
| 16 | Product text search | sunilkumarc/product-hunt-cli search | `search <query>` (FTS offline) + `posts search <query>` (API) | Offline FTS5 after sync; `--json`, `--select` |
| 17 | Maker/author search | sunilkumarc/product-hunt-cli author search | `users search <name>` | Offline FTS on name+username |
| 18 | Offline SQL access | N/A (novel) | `sql "<query>"` (generated) | Full SQLite query access to synced data |
| 19 | Follow/unfollow user | GraphQL mutation `userFollow`/`userFollowUndo` | `users follow/unfollow <username>` | Typed exit codes; `--dry-run` |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Launch-day momentum tracker | `posts momentum <slug>` | 10 | Fetches live vote+comment count via GraphQL `post(slug:)`, diffs against last stored snapshot in SQLite, prints delta + elapsed time since snapshot | Founders consistently cite "no dashboard on launch day" as top pain point; no existing CLI has live-diff capability |
| 2 | Competitor audit per topic | `topics audit <topic-slug>` | 10 | Queries posts by topic (VOTES desc) from API, joins synced maker table to flag shared makers across top posts; outputs ranked table with maker overlap count | Founders research "what's already on PH for my category" before launch; no existing CLI covers this as a named command |
| 3 | Daily digest | `digest [<topic-slug>] [--yesterday]` | 10 | Reads local SQLite for posts synced last 24h (or prior day), formats top N by votes per topic into JSON/table/markdown; works entirely offline after sync | Agent briefing workflow explicit in brief; morning ritual PH use case; top agent automation shape |
| 4 | Topic velocity | `topics velocity <topic-slug> [--weeks 2]` | 9 | Aggregates post count + avg votes per week from synced store, prints week-over-week delta table; pure SQLite math | Investors/developers tracking category momentum; requested in GitHub issues; not in any existing PH CLI |
| 5 | Post vote-rate (age-decay) | `posts vote-rate [--topic <slug>] [--days 30]` | 9 | Computes `votes ÷ days-since-launch` per synced post; ranks by vote-rate not raw votes, surfacing recently-launched products punching above weight | Community discussions on "launch momentum vs accumulated votes"; surfaces underrated recent launches |
| 6 | Trending topic detector | `topics trending [--days 7]` | 9 | Groups synced posts by topic, computes growth ratio (this window vs prior window); ranks topics by growth ratio; pure SQLite aggregation | Developers/investors want "what categories are heating up"; no existing PH CLI exposes this |
| 7 | Watchlist | `watchlist add <slug>` / `watchlist remove <slug>` / `watchlist refresh` | 10 | `add` stores post slug in local JSON; `refresh` queries each via GraphQL `post(slug:)`, updates store, prints current stats + delta; `remove` drops entry | "Track my own post" + "track competitor posts" = top-3 use cases in brief; no existing tool has a named watchlist |
| 8 | Topic subscription digest (new-only) | `topics subscribe <slug>` / `topics unsubscribe` / `topics inbox` | 10 | `subscribe` stores topic slug + last-seen cursor; `inbox` fetches posts newer than cursor for each subscribed topic, prints them, advances cursor; never re-shows seen posts | "Only show me what's new since I last checked" is a distinct workflow from full sync; not in any existing PH CLI |
| 9 | Cross-topic post finder | `posts cross-topic <slug1> <slug2> [...]` | 7 | SQLite JOIN on `post_topics` for posts appearing under all specified topic slugs simultaneously; non-trivial for SQL-non-proficient users | "Find AI tools that are also productivity tools"; not in any existing CLI; not a trivial query |
| 10 | Maker portfolio summary | `makers portfolio <username>` | 7 | Fetches user posts via GraphQL `user(username:)` → posts connection; aggregates total votes, avg per launch, best launch, days-since-last; API-driven with store cache | Founders track cumulative impact across all their products; not in any existing PH CLI |
