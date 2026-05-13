# SurgeGraph CLI Brief

## API Identity
- **Domain:** AI-search content operations. SurgeGraph monitors how a brand surfaces in LLM answer engines (AI Visibility), runs topic/domain research, generates and optimizes articles with AI, grounds writing in knowledge libraries, and publishes to WordPress.
- **Users:** SurgeGraph customers running content operations against AI search. Personas: content strategists tracking AI Visibility metrics, content ops generating + publishing at scale, SEO leads diffing topic gaps weekly.
- **Data profile:** Projects own everything. Per project: AI Visibility prompts (date-bucketed runs with citations, sentiment, topic gaps, traffic), writer documents (full + optimized), knowledge libraries, topic researches with hierarchical micro-topics, domain researches. Org-level: team, usage, API keys (OpenAI/Gemini/Anthropic), CMS integrations, brand mentions.
- **Surface:** 69 operations across 10 tags. The spec is a thin REST facade auto-generated from the existing MCP tool surface (`apps/mcp/scripts/export-openapi.ts`), so every operation is a `POST /v1/<tool_name>` with a JSON body. Auth is OAuth 2.1 bearer (Authorization Code + PKCE + Dynamic Client Registration against `https://mcp.surgegraph.io`).
- **Servers:** `https://mcp.surgegraph.io` (prod), `http://localhost:3010` (local dev).

## Reachability Risk
- **None.** Production endpoint is hosted by the user's own company; the spec was regenerated today (2026-05-12). The CLI is generated from a current internal OpenAPI; auth is a documented OAuth flow described in `MCP_OAUTH_WALKTHROUGH.md`. No web-scraping risk, no bot-protection, no anti-bot WAF. The only access constraint is having a logged-in SurgeGraph account.

## Top Workflows
1. **AI Visibility weekly review.** Pick a project → pull `overview` + `trend` + `sentiment` + `traffic_summary` for the last 7/30 days → spot which brand-tracked topics gained or dropped AI mentions. Today this means clicking through the web UI tab by tab.
2. **Citation source audit.** "Who is citing me for AI-visible queries, and from which domains?" → `citations` (overview) → `citation_own_domain` → `citation_domain` per top citing source. Today: cross-tab pivot in the UI.
3. **Topic-gap → bulk publish loop.** Run a `topic_research` (with optional micro-topic expansion) → `get_topic_map` → identify gaps → `create_bulk_documents` (≤50/batch) → `publish_document_to_cms`. Today: split across two products and a manual handoff to WordPress.
4. **Optimize existing inventory.** Pull WordPress posts (`get_wordpress_categories`, author lists) → `create_optimized_document` per post → review and publish back. Today: per-post UI flow.
5. **Domain research → competitor decomposition.** `create_domain_research` for a competitor → `get_domain_research` for extracted topics → diff against own `topic_map` to find missed coverage.
6. **Operational hygiene.** Doctor: `get_usage` (quota + credits) + `get_team` + `list_tools` to confirm the bearer token, plan limits, and which features are active before queuing work.

## Table Stakes
- Project-scoped commands across every resource (projects own everything; almost every endpoint takes `project_id`).
- All read paths support `--json` / `--select` / `--csv` / `--compact` for piping into agents.
- All write paths support `--dry-run` (preview JSON body, no call) and idempotency where the underlying tool supports it.
- Auth: `auth login --browser` (PKCE + DCR), `auth status`, `auth logout`. Bearer cached in OS keyring or config dir; refresh-token rotation honored.
- Doctor command: auth check + token TTL + reachability probe + tool list + usage / credit balance.
- Bulk verbs that batch (`docs create --bulk`, `prompts create --stdin`).
- Pagination flags on every list endpoint with `--limit` + cursor passthrough.
- `mcp call <tool> --json-args '{}'` escape hatch so the CLI is a strict superset of the raw MCP surface (covered by the runtime Cobratree walker for any operation we don't promote to a typed verb).

## Data Layer
- **Primary entities to persist locally:** projects, ai_visibility_prompts (joined with daily runs), ai_visibility_citations, ai_visibility_traffic_pages, writer_documents, optimized_documents, knowledge_libraries, knowledge_library_documents, topic_researches (with topic tree), domain_researches, brand_mentions, wordpress_integrations, wordpress_categories, wordpress_authors.
- **Sync cursor:** per-resource `last_synced_at` + per-project scope. AI Visibility data is date-bucketed so a `date >=` cursor plus project_id is enough for incremental pull. Documents have `updated_at`; topic researches have status (start → progress → complete) suitable for polling.
- **FTS:** Index `ai_visibility_prompts.text`, `ai_visibility_citations.url + snippet`, `writer_documents.title + html_excerpt`, `topic_research.macro/micro topic names`. This is the path to `surgegraph search "AI search optimization"` returning hits across prompts, citations, drafts, and topic-map nodes in one call.
- **Aggregations that the API alone can't cheaply produce:** weekly delta of AI Visibility metrics per brand, citation-domain rank changes, topic-coverage drift, projects-by-stale-doc count, prompts that lost mentions month-over-month, "which knowledge library is most referenced in citations". These are the transcendence wedge.

## Codebase Intelligence
- Source: user-provided `documentation/printing-press/README.md` + `apps/mcp/scripts/export-openapi.ts` + `documentation/printing-press/openapi.json`.
- Auth: HTTP bearer; token issued by `https://mcp.surgegraph.io/token`; CLI must implement OAuth 2.1 Authorization Code + PKCE + Dynamic Client Registration. Walkthrough at `apps/mcp/MCP_OAUTH_WALKTHROUGH.md` and design at `apps/mcp/MCP_OAUTH_PLAN.md`.
- Data model: every domain object hangs off `project_id`; the org owns team/usage/api-keys/cms-integrations.
- Rate limiting: not declared in spec; assume soft per-tenant quota; add adaptive limiter on the client side and surface `cliutil.RateLimitError` on 429.
- Architecture: API is a thin Hono REST wrapper over the MCP tool surface. Every REST op is one MCP tool call, so the CLI inherits the MCP tool contract exactly. There is no GraphQL, no streaming, no webhook receive.

## User Vision
The user has not shared an in-session vision beyond "let's go". The product README under `documentation/printing-press/README.md` already states their goals verbatim:
> "SurgeGraph is not just an SEO content writer. It is an AI-search content operations cockpit: every project, generated article, AI visibility prompt, citation, topic gap, knowledge source, and CMS publish event is a signal about which content actions will improve discoverability in search and AI answers."
This is the framing the CLI README headline should adopt. Target binary names are already authored by the user: `surgegraph-pp-cli` and `surgegraph-pp-mcp`.

## Product Thesis
- **Name:** `surgegraph-pp-cli`. Display name: **SurgeGraph**.
- **Why it should exist:** Today the SurgeGraph workflow is split between two products and a UI cockpit. Power users — and the agents working alongside them — want one cli + MCP surface where they can run topic research, queue bulk articles, publish to WordPress, and diff AI Visibility trends without clicking. The CLI compounds on top of the API because it has a local SQLite that the UI lacks: weekly visibility deltas, citation rank changes, knowledge-library impact on AI traffic, and "what changed since I last ran sync" become one-shot commands.
- **Vs the SurgeGraph web app:** The web app is the source of truth, but it cannot answer "which prompts lost AI citations week-over-week", "which topics from my last topic_research got the most AI traffic after I shipped articles", or "which knowledge libraries are over- or under-referenced in citations". The CLI's local cache makes those answers a single query.
- **Vs raw MCP:** Agents can call MCP tools today, but every call is independent. The CLI wraps that surface with typed commands, structured output, offline search, and compound workflows. `mcp call` stays as an escape hatch.

## Build Priorities
1. **Foundation + auth.** OAuth login (PKCE + DCR), token storage, doctor, `auth status`, projects list. This is the unblocker for every other workflow.
2. **AI Visibility read surface.** All 21 ai_visibility GETs as typed commands, grouped under `visibility` (overview, trend, sentiment, citations, citation-domain, citation-own-domain, topics, topic-gaps, emerging-topics, traffic-summary, traffic-pages, response-structure, opportunities, metadata, prompts list/detail/response, config).
3. **AI Visibility write surface.** `prompts create/update/delete` under `visibility prompts`.
4. **Document surface.** List/get/create/update/delete for writer and optimized docs; bulk-create.
5. **Knowledge libraries.** List/create/delete library + documents.
6. **Research.** Topic + domain research start/list/get + topic-map + expansion.
7. **CMS.** WordPress integration inspection + publish.
8. **Misc.** Locations, languages, writer models, brand mentions, image generation, API key management, account/team/usage.
9. **Transcendence (Phase 1.5 will catalog these).** Local-store-only commands: visibility-delta, citation-domain rank changes, topic-coverage drift, knowledge-library impact, stale-doc projection, content-velocity per project, agent-context bundle for handoff to other LLM agents.

## Spec-emission notes
- Auth: pre-generation enrichment will set `x-auth-vars` on `bearerAuth` to declare `SURGEGRAPH_TOKEN` as `harvested` (auth login writes it) since the canonical path is the OAuth flow, not env-var paste.
- Tier routing: single tier, no free/paid split.
- MCP enrichment: 69 endpoints + ~13 framework tools puts the surface around 82 tools — past the 50-tool threshold. Recommend Cloudflare pattern (`mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden`) when authoring the spec overlay.
- Public-param naming: every operation has only a JSON body (POST tools), so `--flag` names come directly from body schema field names. The Phase 1.5/2 audit should still run to verify no cryptic single-letter fields slipped through.
