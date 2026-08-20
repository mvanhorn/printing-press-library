# BotSee CLI Brief

## API Identity
- **Domain:** AI search visibility / Generative Engine Optimization (GEO). Monitors how brands appear in ChatGPT, Claude, Perplexity, Gemini, Grok responses.
- **Users:** Marketing teams, agencies, devs using AI agents (Claude Code, OpenClaw, Hermes). Audiences range 10-person startups → enterprise brand teams.
- **Data profile:** Hierarchical — Sites → CustomerTypes → Personas → Questions → Analyses → {Competitors, Keywords, Sources, KeywordOpportunities, SourceOpportunities, Responses}. Rich pay-per-query telemetry (credits/cost/model coverage).
- **Auth:** Bearer token, prefix `bts_live_`; env conv. `BOTSEE_API_KEY` (canonical). Rate limit 600/min with X-RateLimit-* headers + Retry-After.
- **Server:** `https://botsee.io`. 56 operations, 43 paths, OpenAPI 3.0.3, components.securitySchemes.BearerAuth.

## Reachability Risk
- **None.** `GET /api/v1/pricing` returned 200 with valid JSON. `POST /api/v1/auth/validate` returned 401 (well-behaved missing-token). No bot protection, no CF challenge, no degraded shapes. Standard HTTP transport.

## Top Workflows
1. **Weekly visibility review** — pull latest analyses across all sites, scan competitor share-of-voice, identify gaps. 60-90 min ritual the BotSee blog explicitly designs for.
2. **One-shot site audit** — create site → generate customer types → personas → questions → run analysis → review results. The bootstrap flow.
3. **Drift detection / alerting** — compare current vs prior analysis for same site/question; surface meaningful visibility swings the dashboard doesn't expose programmatically.
4. **Opportunity mining** — pull keyword_opportunities + source_opportunities across recent analyses; rank what to act on by frequency × competitor coverage.
5. **Executive scorecard** — 5-minute readable rollup across multiple sites: total mentions, top 3 competitors, citation completeness, week-over-week deltas.
6. **Query library management** — track 30-50 high-intent queries with ownership and mapped URLs across personas.
7. **Content gap → publish loop** — generate blog post from analysis findings (`POST /analysis/{uuid}/content`), pipe to local file, ship to CMS.

## Table Stakes
- Parallel multi-LLM analysis (OpenAI, Claude, Perplexity, Gemini, Grok)
- Full CRUD on Sites / CustomerTypes / Personas / Questions
- Analysis lifecycle: kick off, poll status, fetch all 6 result types
- LLM-powered generation (customer types, personas, questions, recommendations, content)
- API key management: list, create, rotate, reset (one-time token), revoke
- Billing: settings (auto-recharge, thresholds), USDC top-up via x402, signup via CC/USDC
- Webhooks: list, create, delete, test (synthetic event), event catalog
- Usage & rate-limit budget endpoints
- JSON output, structured exit codes, --dry-run on every mutation
- Pricing endpoint (no auth required)

## Data Layer
- **Primary entities:** sites, customer_types, personas, questions, analyses, api_keys, webhooks
- **Analysis result tables (one-to-many off analyses):**
  - `analysis_competitors` (name, mentions, sentiment, co_mention_count, model_coverage)
  - `analysis_keywords` (term, frequency, intent_cluster, model_coverage)
  - `analysis_sources` (url, citation_count, domain, first_seen)
  - `analysis_keyword_opportunities` (suggested_query, expected_intent, est_volume)
  - `analysis_source_opportunities` (target_url, gap_reason, competitor_citing_it)
  - `analysis_responses` (model, raw_response_text, completion_time_ms)
- **Sync cursor:** `analyses.completed_at` per site (analyses are append-only once completed)
- **FTS5:** competitor names, keyword terms, source URLs, raw response text — search across every dimension

## Codebase Intelligence
- Source: OpenAPI 3.0.3 spec direct, plus BotSee marketing blog (workflow patterns), plus published pricing endpoint (live JSON).
- **Auth:** `Authorization: Bearer bts_live_<key>`. Env vars: `BOTSEE_API_KEY` (canonical, recommended). Tokens managed via `/api-keys` endpoints with rotate, reset-via-one-time-token, revoke. (Add `x-auth-env-vars: [BOTSEE_API_KEY]` to BearerAuth scheme so slug-derived `BOTSEE_TOKEN` isn't picked.)
- **Data model:** strict hierarchy. CustomerType requires site_uuid path; Persona requires customer_type_uuid path; Question requires persona_uuid path. Analysis takes a JSON body (likely scope + question_uuids).
- **Rate limiting:** 600 req/min/key. X-RateLimit-Limit/Remaining/Reset headers + Retry-After on 429.
- **Architecture insight:** the cost-multiplier (2x post-completion) means cost predictions matter — an offline cost estimator before running analysis is high-leverage. Per-LLM pricing varies (gemini=2 credits, openai=8 credits).
- **Webhooks:** event catalog endpoint exists (probably analysis.completed, analysis.failed), so a `webhook listen --local` proxy would close the loop for ops teams.

## Source Priority
- Single source — no priority gate needed.

## Product Thesis
- **Name:** `botsee-pp-cli` (CLI), library slug `botsee`.
- **Headline:** Every BotSee operation, plus offline analytics no other GEO tool has: drift detection, opportunity mining, executive rollups across sites — all from a local SQLite cache so agents query at zero credit cost.
- **Why it should exist:** BotSee is uniquely API-first in a dashboard-dominated category (Otterly/Profound/Peec/Promptmonitor/Topify/Gauge are all SaaS UIs). The competitors have no CLI. Even BotSee itself lacks a programmatic-friendly local cache — every weekly review burns credits on read-heavy aggregation that should be free. A CLI with SQLite turns BotSee from "API-first GEO" into "agent-native GEO," opening compound use cases: 4-week visibility deltas, automated alert wiring, multi-site executive rollups, FTS over every cited source ever returned.

## Build Priorities
1. **Foundation (P0):** SQLite store for all 7 primary entities + 6 result-detail tables, sync cursor, FTS5 across competitors/keywords/sources/responses.
2. **Absorb (P1):** Every one of the 56 endpoints as a typed command. Bearer auth wired. Rate-limit handling with Retry-After. CRUD on Sites/CT/Personas/Questions. Full analysis lifecycle (run → poll → fetch 6 result types). API key + webhook + billing/usage operations.
3. **Transcend (P2):** Hand-built novel features the generator can't emit — drift detection week-over-week, cost estimator pre-flight, opportunity heatmap, executive scorecard across sites, query library audit, share-of-voice trends, source-domain authority rollup, competitor reaction tracker.
