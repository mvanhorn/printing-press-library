# AppTweak CLI Brief

## API Identity
- Domain: ASO (App Store Optimization) intelligence — iOS App Store + Google Play
- Users: ASO specialists, mobile growth teams, UA managers, app developers tracking organic + paid keyword performance
- Data profile: app metadata, keyword rankings, category charts, reviews, featured content, paid keyword intelligence, DNA taxonomy

## Reachability Risk
- Low — API at `https://public-api.apptweak.com` returns 401 (auth gate, not block)
- Auth: `x-apptweak-key` header, raw API key, env var `APPTWEAK_API_KEY`
- Credit-based: 403 = insufficient credits; 422 = param validation; no undocumented bot-protection

## Top Workflows
1. **Keyword ranking monitor** — track keyword ranks for owned + competitor apps daily across countries; export CSV for trend analysis
2. **Competitive metadata diff** — pull metadata weekly for self + competitors; diff title/subtitle/description changes to spot ASO experiments
3. **Review mining pipeline** — search reviews by term, rating, date; feed into sentiment/support triage
4. **Category chart tracker** — monitor category position history; correlate with featured events + download spikes
5. **Paid keyword intelligence** — pull competitor bid keywords (Apple Search Ads share-of-voice); cross-reference with organic rank

## Table Stakes (from competing tools)
- ckz/apptweak-mcp: 48+ tools covering all endpoints (TypeScript MCP server)
- Nikita-Guzenko/apptweak-mcp: app analytics + competitor analysis MCP (JavaScript)
- semihcihan/App-Store-Optimization-CLI: 90★ ASO keyword research CLI (no AppTweak)
- ngo275/app-agent: 177★ AI-first ASO tool
- abhi11/apptweak: Go SDK (incomplete coverage)

## Data Layer
- Primary entities: apps (iOS numeric ID / Android package), keywords, countries, categories, DNA segments
- Sync cursor: date-range based (YYYY-MM-DD start/end)
- FTS/search: keyword text search, review text search, app name search
- SQLite tables: keyword_rankings, app_metadata, category_rankings, reviews, top_charts, paid_keywords

## Codebase Intelligence
- Source: ckz/apptweak-mcp TypeScript source analysis
- Auth: `x-apptweak-key` header, raw key (no Bearer prefix), env var APPTWEAK_API_KEY
- Data model: apps array + keywords array + country + device per call; historical = start_date/end_date
- Rate limiting: credit-based (not time-based); 403 = out of credits; iOS numeric IDs, Android package names
- Architecture: two base paths — `/api/public/store` (data) + `/api/public/apptweak` (account/tracked apps)

## Product Thesis
- Name: apptweak-pp-cli
- Why it should exist: No CLI for AppTweak exists. The only agent-native AppTweak access is the MCP server, which has no offline capability, no SQLite persistence, no CSV export, no cross-country fan-out, no keyword basket tracking, and no competitor intelligence workflows. An ASO team needs to run the same keyword rank check across 20 countries for 5 apps — this CLI does that in one command.

## Build Priorities
1. Keyword ranking current + history (core ASO workflow)
2. App metadata current + history (competitor intel)
3. Category rankings current + history (chart monitoring)
4. Reviews displayed + search + stats (review mining)
5. Top charts current + history (chart tracking)
6. Keyword metrics + live search + suggestions (keyword research)
7. Paid keywords + share of voice (ads intelligence)
8. Tracked apps CRUD + credits balance (account management)
9. Novel: cross-country keyword rank fan-out, competitor metadata diff, review sentiment aggregation
