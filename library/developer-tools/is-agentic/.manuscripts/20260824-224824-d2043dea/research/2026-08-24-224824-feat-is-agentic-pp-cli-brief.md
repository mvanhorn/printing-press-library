# Is Agentic CLI Brief

## API Identity
- Domain: `https://is-agentic.com`
- Users: developers, product teams, and AI-agent builders evaluating whether public websites, docs, APIs, and MCP surfaces are usable by agents.
- Data profile: one read-only report resource per public HTTP(S) URL; reports contain a score, label, timestamp, stable report URL, eligible-check count, Essential/Recommended/Bonus breakdowns, and evidence-backed issues.
- Auth: none. The public report API and MCP surface are free and read-only.

## Reachability Risk
- Low. Chrome DevTools loaded the homepage, docs, methodology, OpenAPI, report pages, and the scan flow successfully with HTTP 200/202 responses.
- API contract: `GET /api/v1/report?url=<public-url>`; production origin `https://is-agentic.com`.
- Browser study observed website-only scan internals (`POST /scan/<target>` with a Next action and `POST /api/scan/refresh` returning 202). These are not part of the supported OpenAPI contract and should not be hard-coded as the primary CLI transport.

## Top Workflows
1. Retrieve a completed readiness score for a domain or URL and inspect the highest-priority failures.
2. Produce agent-friendly JSON for scripts, CI checks, dashboards, or an LLM context window.
3. Compare multiple sites locally to find readiness leaders, laggards, and common failure patterns.
4. Track a site's score and issue changes across repeated snapshots.
5. Turn failed/partial checks into a prioritized implementation brief, then rescan from the website when changes are live.

## Table Stakes
- Single-target lookup accepting a domain or full URL.
- Human-readable score bar, score label, breakdown, failures, partial checks, evidence, and recommendations.
- Unchanged structured JSON output for agents and scripts.
- Stable report URL and scan timestamp.
- Honest structured errors for invalid URLs, missing reports, rate limits, and temporary unavailability.
- No credential setup for the public read-only surface.

## Data Layer
- Primary entities: reports keyed by canonical target URL; score buckets; issues keyed by check ID and tier.
- Local store: persist fetched reports and issue snapshots for offline compare/history/portfolio analysis.
- Sync cursor: fetched timestamp plus report `scanned_at`; refresh is explicit to avoid surprising API calls and quota use.
- Search/FTS: issue ID/name/recommendation/details and display target, enabling local search across audits.

## Codebase Intelligence
- Official npm CLI: `is-agentic` v1.0.1, Node >=18, one target plus `--json`/`-j`.
- Official CLI behavior: fetch `/api/v1/report` first; when it returns `report_not_found`, open `GET /api/scan/stream?target=<target>` as SSE, report progress, then retry report lookup up to five times with increasing short delays.
- Error contract: RFC 9457-style `application/problem+json` with `code`, `title`, `status`, `detail`, and `resolution`; documented codes include `invalid_url`, `report_not_found`, `method_not_allowed`, `rate_limit_exceeded`, `report_temporarily_unavailable`, and `api_route_not_found`.
- Limits: 120 requests per client IP per 60-second window; `RateLimit-Policy`, `RateLimit`, compatibility quota headers, and `Retry-After` are documented.
- MCP: Streamable HTTP at `/mcp`; documented read-only tools `is_agentic_get_report`, `is_agentic_get_methodology`, and `is_agentic_get_developer_docs`; discovery via `server.json`, MCP server card, API catalog, AI catalog, and agent-skills index.
- Chrome DevTools evidence: report pages render score, Essential/Recommended/Bonus buckets, an observed agent task journey, evaluator notes, prioritized fixes, and a link to the Ora journey.

## User Vision
- Build a careful CLI for `https://is-agentic.com/`, studying the live site and its agent-facing interfaces before generation.

## Product Thesis
- Name: Is Agentic CLI
- Why it should exist: the official npm CLI is a good single-target renderer, but agents and engineering teams need a durable local audit workspace: fetch many targets, preserve evidence, compare snapshots, search recurring blockers, and emit compact machine-ready remediation context without repeatedly spending API quota.

## Build Priorities
1. Match the official single-target report lookup, human report rendering, JSON mode, and structured error behavior.
2. Add explicit local persistence and repeatable sync/history for report snapshots.
3. Add portfolio comparison, issue clustering, and remediation planning over locally stored reports.
4. Expose a truthful, read-only MCP-compatible command surface through the generated CLI.

## Evidence
- `research/openapi.json` and `research/evidence/openapi.raw`: official OpenAPI 3.1 description.
- `research/evidence/docs.raw`, `methodology.raw`, `server-card.raw`, and `catalog.raw`: raw official docs/discovery captures.
- `research/evidence/npm-package/package/bin/is-agentic.js`: official npm CLI implementation.
- Chrome DevTools MCP snapshots/network captures: homepage, docs, methodology, OpenAPI, example.com report, scan request and refresh request.
