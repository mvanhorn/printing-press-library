# Serply CLI Brief

## API Identity
- Domain: Serply (serply.io), a real-time search results (SERP) API. One key, nine verticals: Google web, Google News, Google Scholar, Google Images, Google Video, Bing web, Google Jobs, Google Shopping (products) and Google Maps.
- Users: AI agent builders who need grounded, current web results; SEO and marketing teams checking rankings by country; researchers pulling papers and news; lead-gen teams mining Maps and Jobs.
- Data profile: Stateless request/response. Each call returns a ranked result list (title, link, description/snippet, position, plus vertical-specific fields such as source and date for News, publication and citation counts for Scholar, place details for Maps). No server-side resources to create or mutate. Cached repeat queries are free.

## Reachability Risk
- None. Official documented API at https://serply.io/docs. All nine verticals verified live with an API key on 2026-09-27 (HTTP 200, non-empty results).
- Auth: `X-Api-Key` header. Canonical env var `SERPLY_API_KEY`. Optional headers: `X-Proxy-Location` (country of the egress proxy, e.g. US, GB, DE) and `X-User-Agent` (desktop or mobile).
- Query form: `GET /v1/<vertical>?q=...&num=...` (query-string form). Maps is the one exception: `GET /v1/maps/search/{query}`.
- Pricing: 2,500 free credits, no card. Prepaid packs never expire. `num` caps near 10 on most verticals.

## Top Workflows
1. Grounded web search for an agent: `web --q "..." --num 5 --json`, pipe titles/links/snippets into context.
2. Rank tracking: where does my domain rank for a keyword in a given country (`X-Proxy-Location`)?
3. Research fan-out: one topic, run web + news + scholar together, get a deduplicated cited brief.
4. SERP change watching: rerun a query later and see which URLs entered, left, or moved.
5. Vertical lookups: Maps places for lead-gen, Jobs listings, Shopping prices, Scholar papers.

## Table Stakes
- One command per vertical with every documented parameter (q, num, start, hl, gl, tbs, ceid, proxy location, device).
- `--json`, `--select`, `--dry-run`, typed exit codes, MCP server exposing every vertical as a tool.
- Google operators (`site:`, quoted phrases) and `tbs` time filters pass through in `q`.
- Competitors: SerpAPI (Python/Node SDKs, no CLI of note), Serper (serper.dev, REST only), Tavily/Exa (semantic search, different index). None ships an agent-native CLI with a local SERP history.

## Data Layer
- Primary entities: search runs (query, vertical, location, timestamp) and their result rows (position, title, link, domain, snippet).
- Sync cursor: none; data is captured per query run.
- FTS/search: local history of past SERPs so `diff` and `rank` can compare runs without spending credits.

## Product Thesis
- Name: serply-pp-cli
- Why it should exist: Serply is the cheapest way to get live Google results with per-country proxies, but today it is only reachable through raw HTTP. A CLI plus MCP server gives agents and SEO users one-line access to all nine verticals, and the local SERP history turns single calls into rank tracking and change detection that the API itself does not offer.

## Build Priorities
1. All nine verticals as typed commands (generated), with the `?q=` form and the Maps path exception.
2. `rank`: position of a domain for a query and location.
3. `serp diff`: compare two stored runs of the same query.
4. `research`: web + news + scholar fan-out into a cited markdown brief.
