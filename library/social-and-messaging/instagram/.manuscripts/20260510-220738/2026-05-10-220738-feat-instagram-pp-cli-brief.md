# Instagram CLI Brief

## API Identity
- Domain: Social media platform (Meta/Instagram) — photos, reels, stories, DMs, analytics
- Users: Content creators, marketers, social media managers, power users, developers
- Data profile: Media objects (posts/reels/stories/carousels), user profiles, comments, DMs, hashtags, insights

## Reachability Risk
- **HIGH** — Two distinct API surfaces with very different risk profiles:
  - **Graph API (official):** Low reachability risk. Requires Meta developer app + business/creator account + OAuth 2.0. Stable and documented. ~20-30 endpoints. Restricted: personal accounts unsupported.
  - **Private/Web API (reverse-engineered):** HIGH reachability risk. Session cookie auth (Chrome import). Multiple 403 reports on instagram-private-api in 2024-2025 (issues #1714, #1580, #889). instagrapi (4.8K ⭐) is actively maintained but reports challenge flows from non-browser IPs. Best-case approach: use Chrome-imported cookies + browser-fingerprint HTTP (Surf transport) to replay web app traffic.
  - **Instagram Basic Display API:** DEAD. Deprecated December 4, 2024. Do not use.

## Top Workflows
1. **Browse feed + save**: Pull home timeline, explore, user posts → local SQLite for offline search
2. **Publish content**: Post photo/video/reel/story/carousel (Graph API for business, web for personal)
3. **Story viewer**: View and download stories before they expire
4. **DM management**: List conversations, read messages, send replies
5. **Analytics/insights**: Engagement rates, follower growth, top-performing posts
6. **Follower management**: Who unfollowed, mutual follows, non-followers

## Table Stakes (absorbed from competitors)
- instagrapi: user profile, media list, stories, DMs, follow/unfollow, like/comment, insights
- supreme-gg-gg/instagram-cli: TUI feed view, post management, story viewing
- mcpware/instagram-mcp: 23 Graph API tools (posts, comments, DMs, stories, hashtags, reels, carousels, analytics)
- instagram-private-api (dilame): feed, user search, story download, highlights
- go-instagram-cli: Stories posting with video segmentation
- ig-dl/ibrahimhajjaj: CDP Chrome proxy, gallery-dl/yt-dlp wrappers for download

## Data Layer
- Primary entities: posts, stories, users, comments, dm_threads, dm_messages, hashtags, insights
- Sync cursor: pagination cursors (next_page token / end_cursor) per collection
- FTS/search: full-text on caption, username, hashtag; structured query by date/type/engagement

## Product Thesis
- Name: instagram-pp-cli
- Why it should exist: Every Instagram CLI either requires a business Meta account (Graph API only tools) or is fragile Python (instagrapi). A Go binary with Chrome cookie import + Surf transport gives personal account users a stable, agent-native CLI with offline search, structured output, and local data persistence — no Python runtime required.

## Build Priorities
1. **Auth via Chrome cookie import** — `auth login --chrome` imports session from local Chrome profile. Foundation for all web-API commands.
2. **Feed + user posts sync** → SQLite store → `search`, `sql` commands work offline
3. **Media download** — photos, videos, reels, stories before expiry
4. **Story viewer** — list + download active stories for followed accounts
5. **Profile intelligence** — follower diff, mutual follows, non-follower detection
6. **Graph API publishing** (optional, if user has business account) — post, reel, story via official API
7. **DM read access** — list conversations, read thread
8. **Analytics** — engagement rate per post, follower growth over time
