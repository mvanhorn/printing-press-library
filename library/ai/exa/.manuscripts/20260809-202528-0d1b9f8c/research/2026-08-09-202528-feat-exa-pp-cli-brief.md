# Exa CLI Brief

## API Identity
- Domain: Exa (exa.ai) — neural web search API for AI agents and developers. Search, content extraction, answer synthesis, similar-page lookup, scheduled web monitors, async agent runs, websets (live-updating web searches), webhooks, imports, events.
- Users: AI/LLM app developers, research agents, content analysts, lead-gen/G2M researchers, journalists, coding agents, voice agents.
- Data profile: Search results (title/url/date/author/highlights/summary/text), extracted page content, LLM answers with citations, monitor runs, agent run outputs, webset items (companies/people/articles/papers with entity props), webhook events, imports. Stateless request/response mostly; monitors/agents/websets/webhooks/imports are durable resources.

## Reachability Risk
- None. Official OpenAPI 3.1 spec at `https://api.exa.ai/openapi.json` (38 paths / 60 endpoints), verified live with the user-provided API key:
  - POST /search → 200 (`latest developments in LLMs`)
  - POST /contents → 200 (example.com text extraction)
  - POST /answer → 200 (capital of France → "Paris" with citations)
  - POST /findSimilar → 200 (empty result set, valid response)
- Auth: `x-api-key` header OR `Authorization: Bearer <key>`. Canonical env var: `EXA_API_KEY`. Spec declares both apiKey (header x-api-key) and http bearer schemes.
- Rate limits: /search 10 QPS, /contents 100 QPS, /answer 10 QPS. Errors: 400/401/402/403/404/409/422/429/500/501/502/503 with `{requestId, error, tag}` shape; /contents per-URL errors in `statuses[]` (HTTP 200 even when individual URLs fail).

## Top Workflows
1. **Semantic web search with content** — `search "query"` with filters (domains, category, date range), get highlights/summary/text back for agent context. Types: auto/fast/instant/deep-lite/deep/deep-reasoning. Streaming for real-time chat.
2. **Extract content from known URLs** — batch fetch up to 100 URLs → clean markdown text, highlights, structured summaries, subpages. Always check `statuses` for per-URL failures.
3. **Get an LLM answer with citations** — ask a question, get answer + grounded citations, optional JSON-schema structured output (outputSchema), optional streaming.
4. **Find similar pages** — feed a URL, get semantically similar documents; "more like this" workflow.
5. **Schedule recurring web research (Monitors)** — create monitors with cadence (hourly/daily/weekly), trigger runs, fetch run outputs; batch operations.
6. **Async agent research (Agent API)** — create agent runs with effort/budget/data sources, poll status, stream/list events, cancel, structured outputs.
7. **Websets** — live-updating searches with entity extraction (companies, people, articles, research papers), enrichments, preview, items management.
8. **Webhooks & events** — register webhook endpoints, list delivery attempts/events.

## Table Stakes
- Web search with all filters and content modes (MCP `web_search_exa`, `web_search_advanced_exa` — exa-labs/exa-mcp-server, 4.8k stars)
- Contents/fetch (MCP `web_fetch_exa`)
- Answer with citations
- Find similar
- Deep research (MCP `deep_search_exa`, `deep_research_start/check`)
- Agent runs (MCP `agent_run`)
- Company/people category search (MCP `company_research_exa`, `people_search_exa`, `linkedin_search_exa`)
- Monitors (scheduled searches with webhook delivery)
- SDK wrappers: exa-js (npm), exa-py (PyPI) — official; community: BjornMelin/exa-direct (Python CLI: search/contents/find-similar/answer/research), paperfoot/search-cli (multi-provider search CLI incl. Exa)

## Data Layer
- Primary entities: searches (ephemeral but worth history), results, monitor runs, agent runs, webset items, webhook attempts, events, imports.
- Sync cursor: cursor-based pagination on monitors/runs/websets/imports/webhook attempts; agent runs list; events list.
- FTS/search: local search over synced result history, monitor runs, agent run outputs, webset items.
- Cost tracking: `costDollars.total` on every response — worth persisting per-request for spend dashboards.

## User Vision
- (From user briefing: "Thoroughly study each API, response and then create the CLI with proper testing. Dogfood it too and polish it.") — full coverage of the API surface, live-tested against a real key, polished.

## Product Thesis
- Name: exa-pp-cli
- Why it should exist: Exa's docs/SDKs are request-shaped, not workflow-shaped. A CLI with local history, spend tracking, offline search over past results, monitor/agent run management, and agent-native `--json` output turns Exa into a personal research engine — search once, keep everything, query locally forever. No other Exa tool persists results or tracks per-request cost across sessions.

## Build Priorities
1. search (all types incl. deep + streaming + outputSchema), contents, answer, findSimilar — full request surface, `--json`/`--select`/`--compact`
2. Monitors: create/list/get/update/delete/trigger/runs/batch
3. Agent runs: create (effort, budget, data sources)/list/get/events/cancel/delete
4. Websets: create/list/get/update/delete/preview/items/enrichments/searches
5. Webhooks: create/list/get/update/delete/attempts
6. Imports + events + team
7. Local store: search history, cost tracking, offline search; transcendence commands (spend report, result digests, monitor run diffs)
