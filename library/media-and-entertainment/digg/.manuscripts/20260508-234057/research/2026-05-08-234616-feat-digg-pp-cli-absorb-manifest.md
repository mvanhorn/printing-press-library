# Digg AI Absorb Manifest

## Ecosystem search summary
- **Existing CLIs/wrappers/MCPs for the new Digg AI: zero.** Verified across npm, PyPI, GitHub. The pre-2012 `python-digg` (JeremyGrosser) is a historic artifact for the original Digg API and is not relevant.
- **Adjacent CLIs to inherit conventions from:** `haxor-news` (donnemartin), `hn-cli` (rafaelrinaldi, brianlovin, jbranchaud, heartleo), `circumflex`, `hntop-cli`, `hnterminal`. Reddit-side: `bashtag/reddit-cli`, `mike-lloyd03/reddit-tui`. None of these target Digg.
- **Digg ranking literature:** the original DiggRank (Search Engine Land, 2008) ranked by user diversity + bury ratio. The new Digg AI uses a fundamentally different methodology: cluster-based, with explicit `gravityScore`, `scoreComponents`, time-windowed positivity (`pos6h`, `pos12h`, `pos24h`), and a curated 1,000-AI-influencer pool on X. We do not need to reverse-engineer the algorithm because Digg exposes its inputs and outputs in the page payload (transparency feature).

## User-first persona scan

| Persona | Ritual | Frustration the CLI fixes |
|---|---|---|
| **AI researcher** (skim what shipped on X today before standup) | Opens di.gg/ai once a morning; clicks Rising; reads TLDRs | Yesterday's feed gets overwritten. No way to see "did GPT-X eject from rank 1 overnight?" without remembering. CLI keeps history; `digg replaced --since 24h` answers the exact question. |
| **AI investor / scout** (track which announcements the smartest accounts amplify) | Watches the Digg AI 1000 leaderboard for movement | Web UI shows current rank only. CLI's `digg author <handle>` and `digg climbers --since 1h` surface movement and attribution. |
| **AI agent / pipeline operator** (consuming Digg as a signal) | Polls /ai every N minutes for changes | No structured API. CLI's `--json` + `digg sync` + `digg sql` lets agents query a local SQLite store with the same data. |
| **News-cycle journalist** (cross-reference what's hot across HN, Techmeme, Digg) | Manually checks 3 sites | Digg's payload already contains `hackerNews` and `techmeme` cross-refs per cluster. `digg crossref <id>` exposes them. |

## Absorbed (parity with the HN/news-aggregator-CLI inventory)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | List top stories | haxor-news `hn top`, hn-cli (multiple) | `digg top [--limit N] [--json] [--since 1h]` | Reads from local store; FTS-composable; agent-native flag set |
| 2 | List rising/trending | haxor-news `hn ask`, brianlovin/hn-cli "Rising" tab | `digg rising [--limit N] [--json]` | Sort by `delta` from snapshot history, not just instant rank |
| 3 | Show story detail | hn-cli `view <id>` | `digg story <clusterUrlId>` | Renders TLDR + headline + scoreComponents + author list with their X handles + `replacementRationale` if any |
| 4 | Open story URL in browser | haxor-news `hn open`, brianlovin/hn-cli | `digg open <clusterUrlId>` | Print-by-default + `--launch` per side-effect convention; `cliutil.IsVerifyEnv()` short-circuit |
| 5 | Search stories | brianlovin/hn-cli search | `digg search <query> [--limit N] [--json]` | FTS5 over title + tldr + author username; Digg has no built-in search |
| 6 | Comments / discussion view | haxor-news `hn comments` | `digg posts <clusterUrlId>` | Detail page lists every X post that contributed to the cluster; this is Digg's analog to comments |
| 7 | List by time window | hntop-cli `--time 24h` | `digg today`, `digg yesterday`, `digg since <duration>` | Time-window queries against local snapshot history |
| 8 | --json output everywhere | hn-cli (most variants) | every read command supports `--json` and `--select` | Standard generator-emitted endpoint mirror |
| 9 | --csv export | none in HN-CLI ecosystem | every read command supports `--csv` | Standard generator-emitted output mode |
| 10 | Doctor / health check | hn-cli (none have this) | `digg doctor` | Built-in; checks reachability, store dir, /api/trending/status |
| 11 | Watch / live tail | brianlovin/hn-cli (auto-refresh) | `digg watch [--interval 60s]` | Polls /ai, diffs against last snapshot, prints rank changes; READ-ONLY |
| 12 | Author profile / posts | haxor-news `hn user <username>` | `digg author <username>` | Lists every cluster a given X account contributed to; uses `xId` / `username` from the embedded data |
| 13 | Stats / counters | none have this | `digg stats today` | Wraps `/api/trending/status` `storiesToday` / `clustersToday` / `lastFetchCompletedAt` |
| 14 | Sync / refresh local store | none have this | `digg sync [--full] [--filter top\|rising]` | Standard sync command; persists clusters, authors, snapshots |
| 15 | SQL escape hatch | none have this | `digg sql "<query>"` | Standard SQLite escape hatch over the local store |

## Transcendence (only possible because Digg's payload is this rich, and we have a local store)

| # | Feature | Command | Why only we can do this |
|---|---|---|---|
| T1 | **Replacement archaeology** | `digg replaced [--since 24h] [--limit N] [--json]` | Digg explicitly publishes a `replacementRationale` field for each cluster that was knocked out of rankings. The web UI shows current rankings only — once a story drops, the rationale is gone. CLI reads from local snapshots and surfaces every replacement event with the official rationale. Score: **9/10** — flagship feature. |
| T2 | **Live pipeline tail** | `digg events [--since 1h] [--type cluster_detected\|fast_climb\|post_understanding\|...] [--watch]` | `/api/trending/status` exposes a live stream of pipeline events: clusters detected, stories fast-climbing with explicit delta + previousRank → currentRank, X posts being processed (with handle + permalink), batch breakdowns. The web UI shows none of this. Score: **10/10** — flagship feature, agent-killer use case ("ping me when GPT-X just fast-climbed 10 spots"). |
| T3 | **Score-component breakdown** | `digg evidence <clusterUrlId> [--json]` | Digg exposes `scoreComponents`, `evidence`, `numeratorCount`/`numeratorLabel`, and `percentAboveAverage` per cluster. The web UI shows the final `gravityScore` only. CLI prints the full transparency record. Score: **8/10** |
| T4 | **Sentiment / positivity windows** | `digg sentiment <clusterUrlId> [--window 6h\|12h\|24h] [--json]` | Each cluster carries `pos6h`, `pos12h`, `pos24h`, `posLast` — per-time-window positivity ratios. No web UI surfaces this. Useful for "is this story still trending positively or has the conversation soured?". Score: **7/10** |
| T5 | **Cross-aggregator references** | `digg crossref <clusterUrlId>` | Each cluster carries `hackerNews` and `techmeme` reference fields when Digg detects the same story discussed there. CLI prints all three URLs side-by-side. Score: **8/10** — answers a real cross-research workflow without leaving the terminal. |
| T6 | **Influence-ranked author leaderboard** | `digg authors top [--by influence\|posts\|reach] [--limit 50]` | The Digg AI 1000 — Digg's whole product premise — is a ranked list of 1,000 AI accounts on X with `influence` scores. The web UI surfaces this on a `/ai/influencers` page that's currently 404; CLI exposes it via the embedded author data on every cluster. Score: **9/10** — flagship for the investor/scout persona. |
| T7 | **Rank history per cluster** | `digg history <clusterUrlId> [--json]` | Local snapshots track `currentRank`, `peakRank`, `previousRank`, `delta` over time. Web UI shows current state only; CLI shows the full trajectory ("entered at #18, peaked at #4 over 6h, dropped to #22 by 24h"). Score: **8/10** |
| T8 | **X-influencer stories — alternative view** | `digg author <handle> [--since 7d]` | The data lists every cluster each tracked X account contributed to, with `username`, `xId`, `displayName`, and post type (original / retweet / quote / reply). Lets you see "all stories Scobleizer surfaced this week" at a glance. Score: **7/10** |
| T9 | **Stale-rank alert** | `digg watch --alert "rank.delta>=10"` | Combines T2 events with local snapshot diffs to alert when any cluster moves N+ ranks. Read-only; uses the same printing-press AdaptiveLimiter for its polling. Score: **7/10** |
| T10 | **Pipeline observability dashboard** | `digg pipeline status [--watch]` | One-screen view of `/api/trending/status`: isFetching, nextFetchAt, storiesToday, clustersToday, lastFetchCompletedAt, last 5 events. Power-user dashboard. Score: **6/10** |

**Stub items:** none. Every row above either reads from the embedded data, the `/api/trending/status` endpoint, or the local snapshot store. No external paid APIs, no headless browser at runtime, no Clerk auth dependency.

## Out of scope — REFUSED (read-only ethical scope)

The following features are explicitly NOT in the manifest and will not be built:
- vote / upvote / digg / bury (no mutation)
- bookmark / save / unsave (auth-only mutation)
- comment / reply (auth-only mutation)
- post-as-X (cross-site posting)
- account creation / login automation
- mass-fetch / scrape-everything modes that would generate abusive request rates

Justification: Digg AI's parent platform was shut down in March 2026 over an "unprecedented bot problem" caused by sophisticated AI-agent activity. CEO Justin Mezzell publicly framed AI-agent automation as the existential threat. A read-only CLI that hits `/ai` once per minute (or on `digg sync`) and respects 429s is consistent with the SEO-friendly sitemap entry and the public-by-design payload. A vote/comment/post automator would be exactly what Digg is fighting and would be unethical to ship — even with a `--user-agent` header that identifies it.

## Total feature count
- **Absorbed parity features:** 15
- **Transcendence features:** 10
- **Total commands:** ~25 leaf subcommands

## Local store plan
Tables: `clusters`, `cluster_snapshots` (rank/score history per cluster over time), `authors`, `cluster_authors` (M:N), `events` (one row per /api/trending/status event), `cross_refs` (HN + Techmeme links per cluster). FTS5 view: `clusters_fts(title, tldr, label, source_url)`.
