# Browserbase CLI Brief

## API Identity
- **Domain:** Cloud-hosted headless browser infrastructure (Browser-as-a-Service). Run Puppeteer/Playwright/Selenium scripts against ephemeral cloud browsers; manage sessions, contexts, downloads, recordings, agents (AI browser agents), hosted functions, web fetch/search.
- **Users:** AI agents and engineers building browser automation, web scraping, AI browsing agents (Browserbase powers Stagehand and AI agents), QA engineers, data collectors. Heavy CLI/agent-native users.
- **Data profile:** Sessions (ephemeral, short-lived, ~20-100s), projects (usage quotas), downloads (files), recordings (rrweb/MP4), contexts (persistent browser state), agents + agent runs (AI-driven), functions (hosted JS/TS with versions/invocations). High write-to-read ratio on sessions; moderate-volume metadata (projects, sessions lists) worth caching.

## Reachability Gate
- **PASS.** Probe: `GET https://api.browserbase.com/v1/projects` with `X-BB-API-Key` → HTTP 200, real JSON (project id `1fbe3566-...`, name "Production project", defaultTimeout 300, concurrency 3). `GET /v1/sessions` → 200 `[]`. Without key → 401 `{"error":"Unauthorized","message":"Missing x-bb-api-key header"}` — expected auth behavior. No bot protection.

## Reachability Risk
- **None.** Official OpenAPI v3 spec (openapi.v1.yaml, 200 OK, 3463 lines, ~48 operations). API is a paid SaaS with clean REST; no bot protection on api.browserbase.com. Auth is a simple `X-BB-API-Key` header. Community SDKs are active (sdk-node, sdk-python, both Stainless-generated and maintained). No 403/blocked issues found in research.
- Rate limits per research: `/v1/search` 2 req/sec, `/v1/fetch` 5 req/sec; session-creation caps 5/25/50/150 per minute by plan; Free plan 1hr session / 3 concurrent / 5 sessions-per-min. 429 responses surface rate limiting.

## Top Workflows
1. **Create a session, run a browser script against it, then clean up** — the core loop. `sessions create` → connect (WebSocket/HTTP) → run Puppeteer/Playwright → `sessions stop` (explicit REQUEST_RELEASE, since keepAlive sessions orphan if not released — pain point #1).
2. **Debug a finished session** — get live debug URLs while running; after end, pull session logs, rrweb recording, and page-by-page MP4 downloads for forensics.
3. **Scrape without writing browser code** — `fetch <url>` (raw/markdown/json+schema) and `search web` for research/scraping pipelines. Agent-friendly structured output.
4. **Manage AI browser agents** — create an agent (systemPrompt + resultSchema), run it, poll run messages/status, stop runaway runs.
5. **Operational dashboarding** — list sessions across projects, monitor project usage (browserMinutes, proxyBytes), track downloads, upload extensions/certificates/contexts.

## Table Stakes
- Session lifecycle: create / list / get / update (REQUEST_RELEASE) / stop / debug URLs / logs / recording / replay / upload file
- Contexts: create / get / delete (persistent browser state)
- Downloads: list / get / delete
- Extensions & certificates: upload / get / list / delete
- Projects: list / get / usage
- Fetch API: raw / markdown / json+schema
- Web Search: query + numResults (1-25)
- Agents: create / list / get / update / delete; AgentRuns: create / list / get / messages / stop
- Functions: list / get / versions / invocations / logs / invoke / builds

## Competitors & Tools (absorb source)
- **`browse` CLI (Browserbase/Stagehand)** — the one to beat. Named-session daemon (`--session`), 30+ DOM commands (open/snapshot/click/fill/type/select/press/upload/highlight/mouse/wait/tab/network/eval/get url/title/text/markdown/screenshot/doctor), full cloud surface (`cloud projects/sessions/contexts/extensions`, `cloud fetch`, `cloud search`), `functions init|dev|publish|invoke`, templates, skills install. DOM-interaction layer is browser-session-based (connectUrl + CDP) — that's a heavier surface; our CLI should absorb the *cloud management* surface + fetch/search, and can provide `session connect` for the connectUrl.
- **Steel CLI** — named-session lifecycle, `scrape|screenshot|pdf`, agent-browser passthrough, templates.
- **Bright Data CLI** — `scrape`, `search`, `discover`, `scraper create|run|heal`, `pipelines`, `browser` daemon (open/snapshot/click/type/fill), `zones`, `budget`.
- **MCP server (Browserbase)** — 6 tools: start, end, navigate, act, observe, extract (all reattachable by sessionId).
- **Official SDKs** — sdk-node + sdk-python: sessions (create/retrieve/update/list/debug), downloads, logs, recording (rrweb + MP4 downloads), replays (HLS), uploads, contexts, certificates, extensions, projects, search.web, fetchAPI, agents + runs.
- **Pain points (from GitHub issues):** (1) keepAlive sessions orphan if not explicitly released → CLI must own explicit `stop`/REQUEST_RELEASE; (2) no explicit kill method in Python SDK → expose `sessions stop`; (3) implicit session state desyncs in hosted MCP → prefer explicit named sessions.

## Data Layer
- **Primary entities:** sessions, projects, downloads, recordings, agent runs, contexts, extensions, functions (versions/invocations), agents.
- **Sync cursor:** sessions list + projects usage are the highest-value sync targets (metadata, JSON-serializable). Recordings/downloads are heavy (binary) — store metadata + URLs, not bytes.
- **FTS/search:** sessions (id, status, projectId, userMetadata), projects (id, name), agents (name), agent runs (sessionId, status) — searchable via generic resources_fts.

## User Vision
- (User chose "Let's go" — no extra context beyond: build it, test deeply, dogfood with the live key.)

## Product Thesis
- **Name:** `browserbase-pp-cli`
- **Why it should exist:** The incumbent `browse` CLI is a Stagehand-first DOM daemon; there is no clean, agent-native, offline-capable CLI for managing the *full* Browserbase cloud surface. A CLI that owns the session lifecycle (create → connect → debug → stop), provides fetch/search/functions, caches projects+sessions locally, and exposes everything as structured JSON with a local SQLite store beats both the raw API and the heavyweight DOM daemon for the automation/agent use case.

## Build Priorities
1. **Session lifecycle commands** (create/list/get/stop/update/debug/logs/recording/replay/upload) — the core, plus explicit stop for the orphan problem
2. **Projects + usage** (list/get/usage) with local cache
3. **Downloads + recordings** (list/get/delete, MP4 download links, replay metadata)
4. **Contexts/extensions/certificates** management
5. **Fetch + search** (raw/markdown/json, web search with numResults)
6. **Agents + agent runs** (create/list/get/update/delete/run/stop/messages)
7. **Functions** (list/get/versions/invocations/logs/builds/invoke)
8. **Transcendence:** local session history + cost/usage analytics, session health/status dashboard, "what ran in my project this week", orphaned-session detection (running sessions older than X with no activity), fetch/search history with cached results, agent-run diffing across runs.

## Auth
- Header: `X-BB-API-Key` (apiKey scheme, in header). Canonical env var: `BROWSERBASE_API_KEY` (also `BROWSERBASE_PROJECT_ID` for project scoping). Keys look like `bb_live_...`.
