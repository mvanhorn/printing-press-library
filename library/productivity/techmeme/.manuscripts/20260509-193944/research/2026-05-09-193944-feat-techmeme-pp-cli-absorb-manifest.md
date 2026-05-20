# Techmeme CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Current top headlines | techmeme.com/feed.xml | `headlines` | Parsed RSS, --json, --compact, offline cache |
| 2 | 5-day headline archive | techmeme.com/river | `river [--hours N]` | Parsed HTML, time filtering, local SQLite |
| 3 | Source leaderboard | techmeme.com/lb.opml | `leaderboard` | Parsed OPML, ranked list, --json |
| 4 | Search headlines | techmeme.com/search | `search <query>` | Full operator support, RSS or HTML parse |
| 5 | Search RSS feed | techmeme.com/search/d3results.jsp | `search <query> --rss` | Subscribe-compatible output |
| 6 | Homepage stories | techmeme.com/ | `top` | Full story parsing with discussions |
| 7 | Story discussions | techmeme.com/ (discussion links) | `top --discussions` | HN, Reddit, X/Bluesky threads per story |

### Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | What did I miss | `since <duration>` | Time-windowed headlines from river data. "since 4h" shows what happened while you were away. Requires local timestamp index |
| 2 | Topic tracker | `track <topic>` | Save topics, auto-search on every sync. Local alerts when a tracked topic hits Techmeme. No equivalent exists anywhere |
| 3 | Source monitor | `sources [--top N]` | Analyze which sources dominate Techmeme. Track source frequency over time from cached river data |
| 4 | Trending topics | `trending` | Extract most-mentioned terms from recent headlines. NLP-lite frequency analysis on cached headlines |
| 5 | Daily digest | `digest [--date YYYY-MM-DD]` | Summarize a day's news by grouping headlines into topic clusters. Requires local headline history |
| 6 | Story velocity | `velocity` | Show which stories are rising fast — multiple sources covering the same topic in a short window |
| 7 | Author spotlight | `author <name>` | Find all headlines by a specific journalist across the cached archive. Cross-source author tracking |
