# Novel Features Brainstorm — Product Hunt CLI
<!-- Full subagent output saved for retro/dogfood debugging -->

## Customer model

(Reconstructed from subagent output)

**Persona 1: The Launch-Day Founder**
Today (without CLI): Has producthunt.com open in a tab, F5-refreshing every few minutes on launch day. Runs no scripts; watches the rank counter manually. Cannot see velocity — only snapshots.
Weekly ritual: Launches a product or monitors an existing one. Checks PH rank at breakfast, lunch, end of day.
Frustration: No way to see "am I trending UP or DOWN right now without having a browser open."

**Persona 2: The Developer Tracking a Topic (e.g., "developer-tools")**
Today: Visits producthunt.com/topics/developer-tools periodically. Has no memory of what they already saw. Re-reads posts they saw last week. 
Weekly ritual: Checks PH 2-3 times a week looking for new tools in their domain. Often misses launches they would have cared about.
Frustration: "Show me only what's NEW since I last checked" is not a feature PH offers.

**Persona 3: The Pre-Launch Founder Doing Competitive Research**
Today: Searches PH manually, opens 10 tabs, compares vote counts by eye. Has no way to see which makers appear repeatedly across top products in their category.
Weekly ritual: Audits the top products in their category monthly. Tracks whether the category is getting more or less active.
Frustration: No named command for "give me every recent launch in [topic] ranked by votes with maker overlap."

**Persona 4: The Agent / Automation Builder**
Today: Calls the GraphQL API directly for data, but hits rate limits because every script re-queries the same data. Has no offline store.
Weekly ritual: Runs a morning briefing that summarizes yesterday's top launches. Pipes PH data into LLM summaries.
Frustration: Rate limits on every script run; no local store; no cursor-based new-only feed.

## Transcendence Table (Survivors)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Launch-day momentum tracker | `posts momentum <slug>` | 10 | Fetches live vote+comment count via `post(slug:)` GraphQL, diffs against last stored snapshot in SQLite, prints delta + elapsed time | Founder communities cite "no dashboard on launch day" pain; no existing CLI has this |
| 2 | Competitor audit per topic | `topics audit <topic-slug>` | 10 | Queries posts by topic (VOTES desc), joins synced maker table to flag maker overlap across top posts | Founders research "what's already on PH for my category" before launch; no existing CLI covers this |
| 3 | Daily digest | `digest [<topic-slug>] [--yesterday]` | 10 | Reads local SQLite for posts synced last 24h, formats top N by votes per topic into JSON/table/markdown | Agent briefing workflow; every PH power user mentions morning routines; #1 agent use case |
| 4 | Topic velocity | `topics velocity <topic-slug> [--weeks 2]` | 9 | Aggregates post count + avg votes per week from store, prints week-over-week delta | Investors/devs tracking category momentum; not in any existing PH CLI |
| 5 | Post vote-rate (age-decay) | `posts vote-rate [--topic <slug>]` | 9 | Computes votes ÷ days-since-launch per post; ranks by vote-rate not raw votes | Surfaces recently-launched products punching above weight; not in any existing tool |
| 6 | Trending topic detector | `topics trending [--days 7]` | 9 | Groups posts by topic, computes growth ratio vs prior window from SQLite | Developers/investors want "what categories are heating up"; no existing PH CLI |
| 7 | Watchlist | `watchlist add/remove/refresh` | 10 | `add` stores slug locally; `refresh` queries each via `post(slug:)`, updates store, prints stats | "Track my own post" + "track competitor posts" = top-3 brief use cases; not in any tool |
| 8 | Topic subscription digest (new-only) | `topics subscribe/unsubscribe/inbox` | 10 | `subscribe` stores topic+cursor; `inbox` fetches posts newer than cursor, prints, advances cursor | "Only show me what's new since I last checked" workflow; not in any existing PH CLI |
| 9 | Cross-topic post finder | `posts cross-topic <slug1> <slug2>` | 7 | SQLite join on post_topics for posts appearing under all specified topic slugs simultaneously | "Find AI tools that are also productivity tools"; not a trivial SQL query for non-technical users |
| 10 | Maker portfolio summary | `makers portfolio <username>` | 7 | Fetches user's posts via API, aggregates: total votes, avg per launch, best launch, days-since-last | Founders track cumulative impact; not in any existing CLI |

## Killed candidates (audit trail)
- Launch calendar: API doesn't expose scheduled/upcoming posts endpoint
- Launch streak: featuredAt is sparse and unreliable for consecutive-day calculation
- Featured-vs-unfeatured ratio: misleading without complete history
- Weekly leaderboard diff: duplicate of topic velocity
- Notification watcher: polling daemon = scope creep
- Post sentiment proxy: LLM-adjacent framing + covered by upvote-to-comment ratio
- CLI onboarding wizard: covered by generated doctor/auth flow
- Maker co-network graph: verifiability low without graph tooling
- Topic vote leaderboard: subset of topic velocity
- Comment thread tree: terminal rendering fragile, marginal CLI gain
- Maker debut detector: misleading on partial sync
- Upvote-to-comment ratio: covered by SQL for power users
- Related posts: subsumed by SQL+FTS
- Post comparison: niche, covered by two post-get calls
- Collection diff: low pain, implied by SQL
- Hunter leaderboard: niche vanity, one SQL query away
- Bookmark export: covered by watchlist + --output flag
