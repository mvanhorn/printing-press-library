# Instagram CLI — Absorb Manifest

API: Instagram Graph API (Facebook-Login path, `graph.facebook.com/v22.0`). Auth: bearer/access_token. Domain: multi-brand social analytics over owned Business/Creator accounts. Local SQLite store is the moat (cross-account + time-series only exist locally).

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Account profile (username, followers_count, follows_count, media_count, biography, website) | facebook SDK IGUser / instagram-analytics-mcp | (generated endpoint) accounts get | Offline cache, --json/--select, multi-account registry |
| 2 | Account insights (reach, views, accounts_engaged, total_interactions, likes/comments/shares/saves; period=day, metric_type=total_value) | instagram-analytics-mcp | (generated endpoint) account-insights list | Correct v22 total_value shape, snapshot-persisted for history |
| 3 | Follower demographics (lifetime; breakdown age/gender/city/country) | instagram-analytics-mcp | (generated endpoint) account-insights demographics | Breakdown flags, offline cache |
| 4 | Media list (id, caption, media_type, media_product_type, permalink, timestamp, like_count, comments_count) | facebook SDK IGUser.media | (generated endpoint) media list | Cursor pagination, FTS on captions, SQL-composable |
| 5 | Per-media insights (reach, views, saved, shares, total_interactions; Reels watch-time) | instagram-analytics-mcp | (generated endpoint) media insights | Uses `views` (not deprecated impressions); graceful per-type metric skip |
| 6 | Stories list + insights (24h window) | instagram-mcp | (generated endpoint) stories list | Captured into store before the 24h window closes |
| 7 | Comments + replies | instagram-mcp | (generated endpoint) comments list | FTS on comment text, offline |
| 8 | Hashtag search -> id, then top_media / recent_media | facebook SDK | (generated endpoint) hashtag top-media | Caches results, respects 30-tag/7d cap |
| 9 | Business discovery (competitor public followers/media/engagement) | facebook SDK business_discovery | (generated endpoint) business-discovery | Snapshots persisted for delta tracking |
| 10 | Tags / mentions (media you are tagged in) | instagram-mcp | (generated endpoint) tags list | Offline cache, --json |
| 11 | Token / connection health (scopes, expiry, ig-user-id resolution, rate-limit headers) | meta MCPs | (behavior in instagram-pp-cli doctor) | Resolves Page->IG-user-id, parses x-business-use-case-usage, debug_token |
| 12 | Content publish (create container + publish) — write, optional | instagram-mcp | (generated endpoint) media publish | --dry-run, content_publishing_limit check |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Cross-account engagement compare | compare | hand-code | Joins `accounts` + `account_insights_snapshots` to rank every owned brand by reach/interactions/ER over a window — a view that exists nowhere in Meta's UI or any API call | Use to compare/rank your OWNED brand accounts on engagement over a window. Do NOT use for follower count trends (use 'growth') or rival accounts (use 'rivals'). |
| 2 | Follower-growth WoW | growth | hand-code | Diffs date-stamped followers_count rows in `account_insights_snapshots`; the store IS the time-series Graph deprecated in 2025 | Use for follower-count growth over time per brand. Do NOT use for engagement/ER (use 'compare'). Requires accrued snapshots — empty on a fresh store. |
| 3 | Best-time-to-post | best-time | hand-code | Buckets each post's total_interactions by weekday/hour of media.timestamp across media+media_insights; Graph has no native best-time | none |
| 4 | Top-posts ranking | top-posts | hand-code | Ranks media across all brands by a chosen metric (reach/interactions/saved/shares) in one windowed local query | Use to rank individual posts by a performance metric. Do NOT use for format-mix analysis (use 'formats') or account-level ranking (use 'compare'). |
| 5 | Format performance breakdown | formats | hand-code | Groups media by media_product_type (Reel/Feed/Story/Carousel) and aggregates reach/ER/Reels watch-time — no native breakdown exists | Use to compare performance by content format. Do NOT use to rank individual posts (use 'top-posts'). |
| 6 | Competitor delta over time | rivals | hand-code | Diffs `competitors` (business_discovery) snapshots across syncs into rival growth/engagement deltas benchmarked vs your brands — point-in-time API made into a trend by the store | Use for tracking rival public accounts' growth over time. Do NOT use for your own brands (use 'compare'/'growth'). |
| 7 | Hashtag-performance ranking | hashtag-perf | hand-code | Ranks tracked hashtags by aggregated hashtag_media reach/engagement from the local store, respecting Graph's 30-tag/7d caps | Use to rank hashtags you track by performance. Do NOT use to discover new hashtags (use 'hashtag top-media'). |

Killed (audit trail in novel-features-brainstorm.md): engagement-rate report (→compare), comment sentiment (LLM), AI caption recs (LLM), cadence consistency (→best-time), demographics-compare (weekly-use fail), story completion-rate (verifiability), saves/shares leaderboard (→top-posts), portfolio digest (composable), stale-account health (→doctor).

## Stubs
None. `media publish` (absorbed #12) ships as a real generated write endpoint with `--dry-run`, but is de-emphasized (analytics-first CLI).
