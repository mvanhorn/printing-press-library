# Exa CLI Brief

## API Identity
- Domain: AI-powered neural web search API for AI agents and developers
- Users: AI engineers building agents, researchers, developers doing web research, content teams
- Data profile: Web search results (URLs, titles, text, highlights, summaries, entities); content extraction (cleaned text from URLs); grounded answers with citations; semantic similarity; monitors/webhooks for recurring searches; async agent runs for deep research

## Reachability Risk
- None — Official OpenAPI 3.1.0 spec at https://exa.ai/docs/exa-spec.yaml (18,236 lines)
- API is publicly accessible with standard API key auth
- Base URL: https://api.exa.ai
- Probe-safe endpoint: POST /search (with minimal body)

## Spec Source
- URL: https://exa.ai/docs/exa-spec.yaml
- Format: OpenAPI 3.1.0
- Coverage: /search, /contents, /answer, /monitors, /agent/runs, /v0/websets, team management

## Auth
- Type: API key in header `x-api-key: <key>`
- Also accepts Bearer token: `Authorization: Bearer <key>`
- Canonical env var: EXA_API_KEY
- Obtain: https://dashboard.exa.ai/api-keys

## Top Workflows
1. **Neural web search with content extraction**: Search for a topic, get full text/highlights/summaries of top results in one call
2. **Grounded Q&A**: Ask a question, get an LLM-powered answer with live web citations
3. **URL content batch extraction**: Get cleaned text/summaries from a list of known URLs
4. **Semantic discovery**: Find pages similar to a known URL
5. **Research monitoring**: Set up recurring searches with webhook delivery for news/topics

## Table Stakes (from competitors)
- parallel-cli: deep async research reports, context chaining, geo-filtering, Boolean queries
- firecrawl: site mapping/crawl, JS rendering, screenshot extraction, multiple output formats
- valyu-cli: multi-source search (academic+finance+patents in one call), domain-specific types
- All: --json output, domain filtering, result counts, file output, --dry-run

## Data Layer
- Primary entities: SearchResult (url, title, text, highlights, summary, publishedDate, author, entities), ContentResult (url, text, highlights, summary, subpages), Answer (answer text + citations), Monitor
- Sync cursor: N/A (search results are ephemeral by default; cache via maxAgeHours=-1)
- FTS/search: Local SQLite store for saved searches, results history, monitors state
- Offline value: Search history with FTS, saved results, cost tracking per query

## Codebase Intelligence
- Source: irona-chat integration + exa-js npm SDK
- Auth: x-api-key header, EXA_API_KEY env var pattern
- Data model: results[] array with {id, url, title, publishedDate, author, text, highlights, highlightScores, summary, entities, extras, subpages}
- Rate limiting: 10 QPS for /search and /answer, 100 QPS for /contents
- Architecture: Simple REST API; POST bodies with JSON; streaming via SSE when stream:true

## Product Thesis
- Name: exa-pp-cli
- Why it should exist: No public CLI exists for Exa.ai despite being the #1 neural search API for AI agents. Every team building with Exa must write their own wrapper (irona-chat did). A CLI would give power users instant access to search, content extraction, grounded answers, and deep research from the terminal — with offline result caching, cost tracking, and agent-native JSON output that beats the SDK's DX.

## Build Priorities
1. Search with full content options (text/highlights/summary) + domain/category/date filters
2. Get contents for batch URL extraction with per-URL options
3. Answer command with citation display + streaming
4. Find-similar for semantic URL discovery
5. Local result store (sync saved searches, search history, cost tracking)
6. Novel: cost-aware search (shows per-query costs, tracks monthly spend)
7. Novel: search digest (trend detection across multiple queries over time)
8. Novel: deep research mode (multi-query synthesis stored locally)
