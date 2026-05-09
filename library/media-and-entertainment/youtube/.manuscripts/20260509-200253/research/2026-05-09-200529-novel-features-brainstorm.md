# YouTube Novel-Features Brainstorm — Audit Trail

## Customer model

### Persona 1: Maya, the channel-tracking researcher
**Today (without this CLI):** Maya monitors ~40 channels covering AI/ML, climate science, and geopolitics for a weekly newsletter. She opens YouTube in a browser, scrolls each channel's "Videos" tab, copies titles into a Google Doc, then opens individual videos to grab transcripts via downloadyoutubetranscripts.com or yt-dlp on the command line. To compare what's trending in her topic vs. the global feed, she eyeballs the Trending tab and tries to remember what was there yesterday.

**Weekly ritual:** Every Friday afternoon she sweeps her watchlist of channels for the week's uploads, pulls transcripts of anything over 10 minutes, searches those transcripts for ~12 topic keywords ("inference", "RAG", "Sora", etc.), and writes up the 5-7 best segments. She also wants to know which trending videos in `US`+`GB` mention her topics so she can spot crossover into the mainstream.

**Frustration:** There is no way to ask "of my 40 channels, which uploaded this week and which transcripts contain `<keyword>`?" without 200+ tab opens.

### Persona 2: Devin, the creator-operator
**Today (without this CLI):** Devin runs a 180k-subscriber tech channel solo. He uses YouTube Studio for analytics, the YouTube API via curl for batch playlist edits, and a homemade Python script that calls `videos.list` to refresh stats on his back catalog. He hits quota limits twice a week.

**Weekly ritual:** Every Monday he refreshes stats on his last 50 uploads, reorders the "Best of 2025" playlist based on view-velocity, replies to top comments on the latest video, and pushes a new thumbnail on a video that under-performed.

**Frustration:** Quota cliffs. He has no way to dry-run a command and see "this will cost 1,400 units of your remaining 6,200" before pressing enter.

### Persona 3: Priya, the agent builder
**Today (without this CLI):** Priya builds a research assistant agent that needs to answer "what does YouTuber X say about topic Y?" She glues together a transcript scraper, the YouTube Data API, and a vector DB.

**Weekly ritual:** She tests the agent against ~30 fixed questions weekly, regenerates the corpus when a tracked channel publishes, and watches latency/cost metrics.

**Frustration:** Every existing MCP server is read-only metadata or transcripts in isolation. None lets her agent SQL-join "videos by channel A in last 90 days WHERE transcript matches `<term>`" in one tool call.

## Survivors (transcendence table)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Quota cost planner | `quota plan <cmd...>` | 10/10 | Parses planned command tree, sums each endpoint's documented unit cost, compares against local 24h ledger keyed by API key | Brief Workflow #6; 10K/day ceiling, search.list=100u |
| 2 | Channel digest with FTS | `digest <channel> --since 7d --keywords k1,k2` | 9/10 | SQLite join of videos × transcripts × channels, filtered by publishedAt > since and FTS match | Brief Workflows #2/#3/#5; Maya |
| 3 | Cross-corpus FTS search | `corpus search "<query>"` | 10/10 | FTS5 union across videos(title+desc+tags), transcripts(content), comments(text); returns video_id, channel, ts_offset, snippet | Brief Data Layer (FTS5 indexes); unique vs every reviewed MCP |
| 4 | Trending list diff | `trending diff <region> --since YYYY-MM-DD` | 8/10 | Reads two rows from trending_snapshots and emits entered/exited/moved lists for (region, category) | Brief Data Layer (trending_snapshots); absorb #8 only snapshots |
| 5 | Per-video velocity | `velocity <channel> --window 30d` | 8/10 | Δviews/Δlikes/Δcomments per day per video from successive videos snapshot rows | Brief Workflow #2 ("tracked over time"); Devin Monday ritual |
| 6 | Topic-trending crossover | `topic crossover "<keyword>"` | 7/10 | Joins trending_snapshots × videos × transcripts and ranks rows whose title/description/transcript match | Brief Data Layer; Maya crossover need |
| 7 | Subscription feed rebuild | `subscriptions sweep --since 7d` | 8/10 | subscriptions.list (OAuth) → for each channel pull uploads playlist crawl from store → emit chronological feed | Brief Users (channel trackers); absorb #14 only lists subs |
| 8 | Quota ledger audit | `cost ledger [--last 24h]` | 8/10 | Reads local quota_log sidecar table every command writes; aggregates by command/endpoint/day | Brief Workflow #6 + Codebase Intelligence |

## Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|------------|---------------------------|
| C8 Comments digest (top by likes) | Thin `ORDER BY likes` — too close to absorb #9 | C3 corpus search |
| C9 Transcript grep | Thinner alias of cross-corpus FTS | C3 |
| C11 Watch Later import | Watch Later not reliably mutable via Data API v3 | C10 subscriptions sweep |
| C13 Playlist sync (diff & apply) | Overlaps absorb #17 playlist CRUD | absorb #17 |
| C15 Engagement outliers | Overlaps absorb #29 engagement analyzer | absorb #29 |
| C16 Caption languages report | Fails weekly-use bar | (none — niche) |
| C14 Channel timeseries | Same family as C5 velocity, coarser | C5 velocity |
