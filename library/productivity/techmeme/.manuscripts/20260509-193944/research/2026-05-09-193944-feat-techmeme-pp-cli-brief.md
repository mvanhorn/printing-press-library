# Techmeme CLI Brief

## API Identity
- Domain: Tech news aggregation and curation
- Users: Tech professionals, journalists, VCs, founders, analysts, AI agents monitoring tech news
- Data profile: Real-time curated headlines, 5-day rolling archive, source leaderboard, full-text search, social reactions
- Base URL: www.techmeme.com
- Auth: None required — all endpoints publicly accessible
- Data format: RSS 2.0 (feeds), OPML (leaderboard), HTML (homepage, river, search)

## Reachability Risk
- None — all 5 endpoints return HTTP 200 via direct curl. No Cloudflare, WAF, or bot detection.

## Available Data Surfaces
1. `/feed.xml` — Top 15 headlines (RSS 2.0), title + description HTML + source + pubDate
2. `/river` — 150+ headlines over 5 days (HTML), timestamp + author + source + headline + link
3. `/lb.opml` — Top 51 sources (OPML), source name + website + RSS feed URL
4. `/search/query?q=<term>` — Full-text search (HTML), supports operators (AND/OR/NOT, sourcename:X, author:X)
5. `/search/d3results.jsp?q=<term>` — Search results as RSS
6. `/` — Homepage (HTML) — richest data: top stories with discussions, social reactions, HN/Reddit threads

## Top Workflows
1. **Morning scan** — What's the top tech news right now? Quick headline scan
2. **Topic monitoring** — Track a specific topic (AI, crypto, Apple) over time
3. **Source analysis** — Which publications dominate tech coverage? Leaderboard insights
4. **Story deep-dive** — Get a story with all its discussion links and social reactions
5. **Catch-up** — What happened in the last N hours/days?

## Table Stakes
- Read current headlines (any Hacker News client does this for HN)
- Search past headlines
- Track specific topics
- View source rankings

## Data Layer
- Primary entities: Headlines (title, source, author, link, timestamp), Sources (name, URL, feed URL, rank)
- Sync cursor: RSS pubDate for feeds, river page for full 5-day archive
- FTS/search: Headline text, source names, author names

## Competitor Landscape
1. **Hacker News CLI tools** — Multiple (hn-cli, hackernews-cli) — different source but similar use case
2. **newsboat** — Terminal RSS reader — generic, not Techmeme-specific
3. **No Techmeme-specific tools exist** — zero CLIs, zero MCP servers, zero SDK wrappers

## Product Thesis
- Name: Techmeme CLI — Tech news intelligence for your terminal
- Why it should exist: Techmeme is the go-to source for curated tech news. There is zero programmatic tooling for it. A CLI that syncs headlines to SQLite enables historical analysis, topic tracking, source monitoring, and "what did I miss" workflows that the website can't provide. For AI agents, this is the single best source of "what's happening in tech right now" — compact, curated, and authoritative.

## Build Priorities
1. RSS feed parsing — headlines with source attribution
2. River parsing — full 5-day headline archive  
3. Search — topic-based headline search
4. Leaderboard — source ranking from OPML
5. Local SQLite with FTS for offline search and historical tracking
