# Parallel.ai CLI Brief

## API Identity
- Domain: Web research infrastructure for AI agents — Search, Extract, Task/Deep Research, FindAll entity discovery, Monitor, Chat; plus Account APIs for balance, apps, and API keys
- Users: Agent builders, RAG/tool-calling developers, research automation operators, platform admins managing prepaid credits/keys
- Data profile: Ephemeral search/extract results; long-lived task runs, findall runs, monitors; local cache of recent searches, run IDs, monitor configs, and balance snapshots

## Reachability Risk
- Low for Product API — live `POST /v1/search` with `x-api-key` returned HTTP 200 (turbo mode, minimal chars)
- Account API cannot be smoked with a standard API key — requires OAuth device-flow Bearer JWT (`Authorization: Bearer <access_token>`)
- Probe-safe endpoint used: `POST /v1/search` (mode=turbo, max_chars_total=200)

## Source Priority
- Primary: parallel-product-api — official OpenAPI complete — auth: `PARALLEL_API_KEY` via `x-api-key`
- Secondary: parallel-account-api — official OpenAPI complete — auth: OAuth device-flow JWT
- **Economics:** Product headline commands only need API key. Account balance/apps/keys gated on OAuth; do not force OAuth for search/extract/tasks.
- **Inversion risk:** Account has fewer endpoints but different auth — do NOT let Account auth bleed into Product commands.

## Top Workflows
1. Agent web search with objective + keyword queries → ranked excerpts (`search`)
2. Extract content from known URLs after search (`extract` + session_id continuity)
3. Deep research / enrichment via Task runs + poll/stream results (`tasks`)
4. Entity discovery (people/companies) via FindAll or fast entity search
5. Continuous monitoring of web changes + webhook/event pull (`monitors`)
6. Admin: check prepaid balance, manage apps/keys (`account`)

## Table Stakes
- Official `parallel-cli` (parallel-web-tools): login/OAuth, search, extract, research, findall, monitor, agent-oriented UX
- `@rikalabs/parallel`: Unix-friendly Search + Extract only
- Official Python/TS SDKs (`parallel-web`): full Product surface
- MCP servers (Search MCP, Task MCP) for IDE agents

## Data Layer
- Primary entities: searches, extracts, task_runs, task_groups, findall_runs, monitors, monitor_events, balance_snapshots, apps, keys
- Sync cursor: task/findall/monitor run IDs + event cursors; search session_id for multi-step agent loops
- FTS/search: local FTS over cached titles/URLs/excerpts and task input/output summaries

## User Vision
- Full CLI covering both API Reference (Product) and Account API docs
- Chrome DevTools–informed contract fidelity
- API key only for testing/checks, not embedded in shipped artifacts
- Printing Press standards + full verify/dogfood/live smoke

## Product Thesis
- Name: `parallel-pp-cli`
- Why it should exist: Beat official CLI on agent-native JSON, offline SQLite recall of past researches, cross-run session continuity, and dual-auth clarity (API key vs Account OAuth) in one Go binary — without requiring Python/Node.

## Build Priorities
1. Product API completeness from `public-openapi.json` (search, extract, tasks, findall, monitors, chat)
2. Account API secondary commands (balance get; apps/keys CRUD) with OAuth device-flow auth path
3. Local store + FTS for searches/runs/monitors; `session` continuity helpers
4. Agent-native `--json`, structured errors, rate-limit hints
5. Novel: research-session stitch (search→extract→task under one local session), balance-aware run cost guard (when OAuth present), stale-monitor digests

## Auth Notes
- Product: header `x-api-key` / env `PARALLEL_API_KEY`
- Account: `Authorization: Bearer <JWT>` from OAuth device flow (see docs `/integrations/account-api`)
- Do not write API key values into source, README, manuscripts, or proofs

## Reachability Gate
- Decision: PASS
- Evidence: POST https://api.parallel.ai/v1/search with x-api-key returned 200 + results (turbo, max_chars_total=100)
- Account API: deferred (requires OAuth JWT; not probed with Product API key)
