# ScrapeCreators CLI Brief

## API Identity
- Domain: Social media data extraction — public profiles, posts, videos, comments, transcripts, trends, and ad intelligence across 27+ platforms
- Users: Growth marketers, influencer agencies, content analysts, brand safety teams, competitive intelligence teams, AI/LLM pipeline builders
- Data profile: Creator profiles (follower counts, engagement), content metadata (videos, posts, reels), comments, transcripts, trending content (songs/hashtags/videos), audience demographics, ad library data, account credits/usage
- Auth: API key via `x-api-key` header; env var: `SCRAPECREATORS_API_KEY`
- Base URL: `https://api.scrapecreators.com`
- Spec: OpenAPI 3.1.0 at `https://docs.scrapecreators.com/openapi.json` (18 TikTok-only paths) — INCOMPLETE vs 100+ endpoints on docs site

## Reachability Risk
- None — HTTP 200 confirmed with valid API key. API responds correctly with structured JSON.

## Top Workflows
1. **Creator intelligence** — fetch a creator's profile + recent videos + audience demographics across TikTok, YouTube, Instagram in one pipeline
2. **Trend tracking** — monitor trending songs/hashtags/videos on TikTok; correlate with YouTube shorts trending
3. **Competitor ad research** — pull Facebook Ad Library, LinkedIn Ad Library, Google Ad Library for a brand's ad campaigns
4. **Content transcript mining** — batch-fetch transcripts from YouTube/TikTok/Instagram videos for topic analysis or RAG
5. **Influencer discovery** — search by hashtag or keyword, filter by engagement rate, build shortlists

## Table Stakes (competitor features to match)
- EnsembleData: multi-platform scraping (Twitter, Instagram, Reddit, Threads) with unit-based billing visibility
- Bright Data: social scraping across Facebook, Instagram, TikTok, YouTube, LinkedIn with structured output
- Data365: 8-year dataset, Facebook/Instagram/TikTok/Reddit/X/Threads
- Phyllo: creator data API, influencer vetting, social screening, 20+ networks
- tiktok-scraper (github.com/drawrowfly): open-source CLI — videos from username/hashtag/trending, batch downloads
- n8n-nodes-scrape-creators: n8n workflow integration with profile fetch, video list, posts

## Data Layer
- Primary entities: creators (cross-platform), content (videos/posts/reels/shorts), comments, trends (songs/hashtags/videos), ad_campaigns, transcripts
- Sync cursor: ISO timestamp per creator per platform (track last sync)
- FTS/search: creators (username, nickname, bio), content (title, description), transcripts (full text), trends (name, platform)

## Product Thesis
- Name: `scrapecreators-pp-cli`
- Why it should exist: The only CLI that spans ALL 27+ ScrapeCreators platforms with offline search, cross-platform joins, transcript FTS, and compound workflows no API call provides — turn single API calls into intelligence pipelines

## Build Priorities
1. Cross-platform creator profile + video sync with SQLite store
2. Transcript sync + full-text search across all synced content
3. Trending data snapshots (TikTok songs/hashtags, YouTube shorts) with time-series in SQLite
4. Ad library commands (Facebook, LinkedIn, Google) for competitive intelligence
5. Cross-platform engagement normalization + growth tracking
6. Account management (credits balance, usage history)
