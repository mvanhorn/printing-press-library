# Keenable CLI Brief

## API Identity
- Domain: `https://api.keenable.ai`; official OpenAPI 3.0.3 spec at `https://docs.keenable.ai/api-reference/openapi.json`.
- Users: AI-agent builders, coding agents, researchers, and developers who need ranked web discovery plus clean page content.
- Data profile: two logical operations, each with keyed and keyless variants: search (`POST /v1/search` and `/v1/search/public`) and fetch (`GET /v1/fetch` and `/v1/fetch/public`). Search returns ranked URLs, titles, descriptions/snippets, publication and acquisition timestamps. Fetch returns URL, title, description, author, publication metadata, and Markdown content.
- Auth: keyed requests use `X-API-Key`; `Authorization: Bearer` is also accepted, with `X-API-Key` taking precedence. Keyless endpoints require `X-Keenable-Title` and use a shared IP pool.

## Reachability Risk
- None observed: authenticated `POST /v1/search` returned `200` with ranked results using the supplied credential; the keyless fetch probe also returned `200` with the documented response shape and `X-RateLimit-Limit: 1000`, `X-RateLimit-Remaining: 999`.
- The credential value is not written to this brief or any artifact. Authenticated live dogfood is now authorized and will remain read-only.
- No browser-sniff was needed after Chrome DevTools MCP study and official OpenAPI resolution; the spec covers the documented HTTP surface.

## Top Workflows
1. Search for current technical, market, or news information with site and date filters.
2. Fetch a known URL as clean Markdown, optionally using `live=true` for uncached pages.
3. Run reproducible point-in-time research using `query_time` and relative/absolute date windows.
4. Extract targeted facts from a page with the `prompt` parameter instead of returning the full document.
5. Feed search results and fetched pages into agent pipelines through JSON/Markdown, MCP, or local research bundles.

## Table Stakes
- Search and fetch commands with agent-friendly structured output.
- Site restriction, publication/acquisition date filters, point-in-time search, snippet length, and result-count controls.
- Keyless fallback with application-identifying header; authenticated key override and environment-variable support.
- Human-readable output, clear API errors, 401/402/403/404/422/429 handling, `Retry-After` awareness, and rate-limit visibility.
- MCP-compatible search/fetch tools and configuration guidance for Claude Code, Cursor, Codex, Windsurf, OpenCode, and stdio-only clients.
- Credential login/logout/configuration in the incumbent CLI, plus configurable search mode and update commands.

## Data Layer
- Primary entities: search runs, search results, fetched pages, extracted prompts, citations, and local research bundles.
- Sync cursor: none; this API is read-through and stateless. Local persistence is valuable for reproducibility, deduplication, offline search, and comparing answers over time rather than for mirroring remote resources.
- FTS/search: SQLite FTS5 over saved result titles, URLs, descriptions, snippets, and fetched Markdown; local queries must never imply fresh upstream data.

## Codebase Intelligence
- Official CLI: `https://github.com/keenableai/keenable-cli` (Rust). Commands include `search`, `fetch`, `login`, `logout`, `configure-mcp`, `reset`, `config`, and `update`; search defaults to agent-oriented YAML and supports pretty output, date/site filters, point-in-time search, mode, snippet/result bounds, and per-call key override.
- Official MCP: `https://github.com/keenableai/keenable-mcp` (JavaScript). It is a thin stdio bridge to `https://api.keenable.ai/mcp`, forwards tools dynamically, and uses `KEENABLE_API_KEY` as an optional `X-API-Key` header.
- Auth: single-token API key, canonical environment variable `KEENABLE_API_KEY`; authenticated usage is per-organization and keyless usage is per-IP.
- Data model: search response is `{query, results[]}`; fetch response is `{url, title, description, author, content, published_at}`. MCP exposes `search_web_pages` and `fetch_page_content`, plus `_meta["keenable/usage"]` for authenticated billing metadata and `_meta["keenable/overrides"]` for mode/cache controls.
- Rate limiting: authenticated requests remove the hourly cap and remain organization-scoped per-second limited; keyless requests are capped at 1,000/hour and 10/second per IP. `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, and `Retry-After` are documented.
- Architecture: official MCP is a remote Streamable HTTP service with a local stdio bridge; the HTTP API remains the stable replayable runtime surface.

## User Vision
- Build and publish a genuinely useful Keenable CLI with many novel features, study the API deeply through Chrome DevTools MCP, use the supplied credential for testing, and complete the full Printing Press pipeline.

## Product Thesis
- Name: Keenable Research CLI
- Why it should exist: The official CLI and MCP expose the raw search/fetch primitives, but agents repeatedly need durable evidence, reproducible snapshots, multi-page research, citation-ready output, and local comparisons. This CLI should preserve the direct API surface while turning individual calls into inspectable research artifacts that compound locally.

## Evidence Sources
- Official API reference: https://docs.keenable.ai/api-reference
- Search contract: https://docs.keenable.ai/api-reference/search
- Fetch contract: https://docs.keenable.ai/api-reference/fetch
- Authentication: https://docs.keenable.ai/authentication
- Rate limits: https://docs.keenable.ai/rate-limits
- Credits: https://docs.keenable.ai/credits
- CLI: https://docs.keenable.ai/cli
- MCP: https://docs.keenable.ai/mcp-server
- Official CLI source: https://github.com/keenableai/keenable-cli
- Official MCP source: https://github.com/keenableai/keenable-mcp
- Comparable agent search/fetch products: Tavily, Exa, Firecrawl, Brave Search, and Perplexity Sonar; they establish table stakes around search, extraction, citations, freshness controls, and agent integrations.
- GitHub issue reachability check: official CLI and MCP repositories currently expose zero GitHub issues; no 403/blocked/deprecated issue cluster was found.

## Build Priorities
1. Generate every official API operation, including keyed and keyless paths, with the canonical `KEENABLE_API_KEY` env var and `X-Keenable-Title` support.
2. Preserve agent-native JSON, bounded result/content controls, date semantics, point-in-time queries, live fetch, and prompt extraction.
3. Add SQLite-backed saved runs, FTS, reproducible snapshots, citation export, and research bundles.
4. Add multi-page fetch and evidence/report commands with bounded concurrency, partial-failure accounting, and rate-limit-safe retries.
5. Expose local research workflows through the generated MCP command mirror without claiming unsupported remote mutations.
