# Apify CLI Brief

## API Identity
- Domain: Web-scraping platform with 4,000+ pre-built Actors, datasets, key-value stores, request queues, schedules, webhooks
- Users: Data engineers, growth/marketing operators, newsletter writers, researchers running recurring scrapes; agents orchestrating ingestion pipelines
- Data profile: Highly heterogeneous (every Actor emits a different shape), high-volume (datasets can be 100k+ items), billed in compute units + RAM-hours + storage

## Reachability Risk
- None. Official OpenAPI spec at https://docs.apify.com/api/openapi.yaml (745 KB, 229 operations, 131 paths). API at https://api.apify.com/v2 is plain HTTPS with Bearer token auth. No bot protection, no Cloudflare challenge, no rate-limited research. GitHub repos public.

## Top Workflows
1. **Daily competitor / sector scan**: schedule cron → run apidojo/twitter-scraper-lite + trudax/reddit-scraper + apify/google-news-scraper in parallel → fetch dataset items → dedupe vs yesterday → markdown digest into newsletter draft
2. **Story-driven deep dive**: pick topic → call harvestlabs/news-aggregator-ai-agent or apify/smart-article-extractor → wait for run → pull clean article bodies → summarize per source → diff against prior runs to surface novelty
3. **Founder / VC signal capture**: weekly apidojo/tweet-scraper V2 over curated handle list → push to dataset → filter by engagement threshold → tag with persona
4. **Cost-aware A/B between scrapers**: same query through kaitoeasyapi vs apidojo → compare output quality + actual CUs billed → pin the winner
5. **Webhook-driven publishing pipeline**: schedule run → webhook fires on ACTOR.RUN.SUCCEEDED → local worker pulls new dataset items → enriches → posts to Beehiiv/Ghost

## Table Stakes (what we MUST match)
- Full CRUD on actors, runs, datasets, kv-stores, request-queues, schedules, webhooks, tasks, builds (the SDK clients cover this; the official CLI does not)
- Multiple dataset output formats (JSON, CSV, XLSX, RSS, HTML)
- Run-and-wait semantics (start a run, block until terminal state, return dataset)
- Webhook subscription management
- Schedule management (cron expressions, enable/disable)
- Account/usage query (`/v2/users/me`, monthly usage)
- Authenticated `--token` from env (`APIFY_TOKEN`)
- Doctor/health check (token valid, API reachable, monthly quota status)

## Data Layer
- **Primary entities**: actors, actor_runs, actor_run_costs (derived), datasets, dataset_items, kv_store_records, request_queues, schedules, webhooks, webhook_dispatches, tasks, builds, input_presets, runs_dedupe_state
- **Sync cursor**: `modifiedAt` on runs/datasets/schedules; `startedAt` for run windows; per-dataset `lastItemId` cursor for incremental item pulls
- **FTS/search**: FTS5 over normalized dataset_items fields (url, title, body_text) + raw JSON; cross-actor query

## Codebase Intelligence
- Source: research subagent + DeepWiki on apify/apify-cli, apify/apify-mcp-server, apify/apify-client-python
- Auth: `Authorization: Bearer <token>` recommended; `?token=` query param legacy; env var `APIFY_TOKEN` (convention across every SDK)
- Data model: REST-ish v2 API with consistent envelope `{ "data": {...} }` for single resources, `{ "data": { "items": [...], "total": N, "limit": N, "offset": N } }` for lists. Errors: `{ "error": { "type": "...", "message": "..." } }`
- Rate limiting: ~30 req/s per IP for unauthenticated, higher for authenticated; 429 with `Retry-After` header. Actor run starts can be throttled by `memoryMbytes` quota
- Architecture: Actors are container images. Runs allocate memory + CPU, produce default dataset/kvs/rq. Dataset items are immutable append-only; downloads paginate

## User Vision
Newsletter tool. Will use this CLI to: scrape competitor moves, AI/dev news, social signals, trend data, feed into a weekly newsletter. The transcendence features should center on: getting newsletter-ready output (deduped, novel-only, formatted) and keeping costs visible across weekly runs.

## Product Thesis
- **Name**: apify-pp-cli (binary), branded as "Apify CLI"
- **Why it should exist**: The official `apify-cli` is for Actor *developers* — `apify init`, `apify push`, `apify call <my-actor>`. The official MCP exposes ~5 narrow tools. No tool today covers the *platform operator's* workflow: orchestrate runs across many Actors, persist results locally, search across runs, track costs, render newsletter-ready output, manage schedules as code. apify-pp-cli is the operator CLI.

## Build Priorities
1. **Priority 0 (foundation)**: full data layer — actors, runs, datasets, dataset_items, kv_store_records, schedules, webhooks, tasks, builds, input_presets, runs_dedupe_state. Sync command. FTS5 over dataset_items. SQL escape hatch.
2. **Priority 1 (absorb table stakes)**: every endpoint group via generated typed commands — actors/runs/datasets/kvs/rq/schedules/webhooks/tasks/builds/users. Run-and-wait. Multi-format dataset export. `doctor`.
3. **Priority 2 (transcendence)**: cross-actor FTS, novelty diffing (`--only-new`), true cost ledger (`cost report`), newsletter digest renderer (`digest`), schedule-as-code (`schedule apply` + `diff`), input presets (`preset save` + `--preset`).

## Critical Notes for Generation
- **Spec size**: 229 operations is at/over the default 20-endpoints-per-resource cap. May need to bump generator caps or accept that some lesser-used endpoints will be elided. Resource priority for keeping endpoints: actors > runs > datasets > schedules > webhooks > kv-stores > request-queues > tasks > builds > store > users.
- **Auth enrichment**: spec uses HTTP Bearer scheme; ensure generator emits `os.Getenv("APIFY_TOKEN")` in config.
- **MCP enrichment**: 229 typed endpoint tools + ~13 framework + ~6 novel commands = ~248 tools. **Strongly recommend Cloudflare pattern** — `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden`. Per the skill, >50 tools should default to this pattern.
- **Tier routing**: Apify is single-tier (one API token covers everything). Do NOT add tier_routing.
- **Public-param-audit**: spec uses descriptive names already (`actorId`, `datasetId`, `runId`, `limit`, `offset`); should pass clean.
- **No browser-sniff needed**: official OpenAPI spec is the canonical source.
- **No crowd-sniff needed**: SDK clients (apify-client-js, apify-client-python) are authoritative for any gaps; already implicit in the spec.
