# SurgeGraph CLI — Absorb Manifest

## Absorbed (1:1 from the SurgeGraph MCP tool surface)

SurgeGraph has no public SDK, no public CLI, and no third-party MCP server. Every absorbed feature is the SurgeGraph MCP tool itself — surfaced as a typed CLI command. The generator emits these from the OpenAPI spec automatically.

| # | Feature family | Best source | Our implementation | Added value |
|---|----------------|-------------|--------------------|-------------|
| 1 | All 21 AI Visibility reads (overview, trend, sentiment, citations, citation-domain, citation-own-domain, topics, topic-gaps, emerging-topics, traffic-summary, traffic-pages, response-structure, opportunities, metadata, prompts list/detail/response, config) | SurgeGraph MCP (visibility.*) | Typed `surgegraph visibility <verb>` with project_id flag | `--json --select --csv --compact`, agent-friendly piping |
| 2 | AI Visibility writes (prompts create/update/delete) | SurgeGraph MCP (visibility.prompts.*) | Typed `surgegraph visibility prompts <verb>` | `--dry-run`, `--stdin` batch, idempotency |
| 3 | Documents (writer + optimized): list/get/create/update/delete, bulk-create | SurgeGraph MCP (writer/optimizer) | Typed `surgegraph docs <verb>` | `--dry-run`, bulk JSON-arg, idempotency |
| 4 | Knowledge libraries + documents: list/create/delete | SurgeGraph MCP (knowledge_library.*) | Typed `surgegraph knowledge <verb>` | Local-store backing |
| 5 | Topic + domain research: start/list/get + topic-map + expansion | SurgeGraph MCP (research.*) | Typed `surgegraph research <verb>` | Polling helpers, hierarchical tree printing |
| 6 | CMS (WordPress): integrations, categories, authors, publish | SurgeGraph MCP (cms.*) | Typed `surgegraph cms <verb>` | `--dry-run` for publish |
| 7 | Content Vision (image gen, gallery, settings) | SurgeGraph MCP (content_vision.*) | Typed `surgegraph vision <verb>` | Async polling, gallery export |
| 8 | API key management (OpenAI / Gemini / Anthropic) | SurgeGraph MCP (api_keys.*) | Typed `surgegraph keys <verb>` | `--dry-run`, scoped redaction |
| 9 | Account: usage, team, tools list | SurgeGraph MCP (account.*) | Typed `surgegraph account <verb>` | Doctor integration, quota check |
| 10 | Misc lookups: locations, languages, writer models, brand mentions, knowledge_libraries listing, project listing | SurgeGraph MCP (misc + project) | Typed commands grouped under `surgegraph projects`, `surgegraph languages`, `surgegraph locations`, `surgegraph models`, `surgegraph brands` | Cache to local store for offline reuse |
| 11 | Raw MCP escape hatch | SurgeGraph MCP | `surgegraph mcp call <tool> --json-args '{}'` | Exposed via the Cobratree walker for ANY operation the user wants by tool name |

**Total absorbed:** 69 endpoints, 100% coverage of the SurgeGraph MCP tool surface.

**Stub list:** none. All 69 endpoints are shipping-scope.

## Transcendence (only possible with our approach)

Survivors of Phase 1.5 Step 1.5c.5 brainstorm + adversarial cut. Full audit trail in `2026-05-12-172017-novel-features-brainstorm.md`.

### Theme: Local state that compounds

| # | Feature | Command | Score | How it works | Persona served |
|---|---------|---------|-------|--------------|----------------|
| 1 | Visibility weekly delta | `visibility delta --project <id> --window 7d [--metric overview,trend,sentiment,traffic]` | 9/10 | Reads two snapshots of `visibility_overview`/`trend`/`sentiment`/`traffic_summary` from local SQLite, emits row-wise deltas. `--metric sentiment` covers the folded-in sentiment-movers case | the content strategist (AI Visibility analyst) |
| 2 | Prompts that lost AI mentions | `visibility prompts losers --project <id> --since 30d` | 9/10 | Joins `ai_visibility_prompts` with date-bucketed daily runs; flags prompts whose citation count or first-position rank dropped | the content strategist |
| 3 | Citation-domain rank shift | `visibility citation-domains rank-shift --project <id> --window 30d` | 8/10 | Diffs two snapshots of `ai_visibility_citations` grouped by domain | the content strategist, the SEO lead |
| 4 | Stale-doc projection | `docs stale --project <id> --older-than 90d` | 7/10 | Local query over `writer_documents.updated_at` joined with `ai_visibility_traffic_pages`, ranked by AI traffic | the content-ops lead, the SEO lead |
| 5 | Cross-project visibility roll-up | `visibility portfolio --window 7d` | 8/10 | Fans the delta engine across every project. UI is one-project-at-a-time; this is the agency view | the SEO lead (6-client agency) |
| 6 | Quota burn-down forecast | `account burn --window 30d` | 5/10 | Linear projection over local snapshots of `get_usage` credit balance | the content-ops lead, the agent-builder |

### Theme: Cross-entity joins no single API call returns

| # | Feature | Command | Score | How it works | Persona served |
|---|---------|---------|-------|--------------|----------------|
| 7 | Knowledge-library impact | `knowledge impact --project <id>` | 7/10 | Joins `knowledge_library_documents.url` against `ai_visibility_citations.url`; ranks libraries by citation reach | the SEO lead |
| 8 | Topic-coverage drift | `research drift --research-id <id> --project <id>` | 8/10 | Joins `topic_research` micro-topic nodes against `writer_documents.title` (FTS-backed); marks covered vs gap | the content-ops lead, the SEO lead |
| 9 | Competitor topic diff | `research domain diff --mine <research-id> --theirs <domain-research-id>` | 7/10 | Set-diff between own topic_research and competitor domain_research topic trees | the SEO lead |
| 10 | Traffic-page → citation join | `visibility traffic-citations --project <id>` | 7/10 | Joins `ai_visibility_traffic_pages.url` against `ai_visibility_citations.url`; lists prompts whose citations drove each top page | the content strategist, the SEO lead |

### Theme: Compound workflows that span products

| # | Feature | Command | Score | How it works | Persona served |
|---|---------|---------|-------|--------------|----------------|
| 11 | Gap → publish pipeline | `research gaps publish --research-id <id> --integration <wp-id> [--dry-run]` | 9/10 | Filter `get_topic_map` for gaps → `create_bulk_documents` (≤50 batches, idempotent) → `publish_document_to_cms`; one command replaces the content-ops ritual | the content-ops lead |

### Theme: Agent-native plumbing

| # | Feature | Command | Score | How it works | Persona served |
|---|---------|---------|-------|--------------|----------------|
| 12 | Agent context bundle | `context bundle --project <id> [--include prompts,citations,docs,topics]` | 7/10 | Reads ≤5 entity tables from local SQLite, emits one JSON blob shaped for agent context windows | the agent-builder |
| 13 | Local search | `search "<query>" [--kind prompts,citations,docs,topics]` | 8/10 | SQLite FTS over `ai_visibility_prompts.text`, `ai_visibility_citations.url+snippet`, `writer_documents.title+html_excerpt`, `topic_research` topic names | the agent-builder, the content strategist |
| 14 | Sync diff | `sync diff --since <ts>` | 6/10 | Per-resource `last_synced_at` cursor + change counts; foundation primitive for any agent loop | the agent-builder |

## Summary

- **69 absorbed features** (100% of the SurgeGraph MCP tool surface, emitted by the generator).
- **14 transcendence features** (hand-built in Phase 3), grouped into 4 themes.
- **4 killed candidates** documented in the brainstorm audit trail (sentiment-movers folded into #1; LLM-dependent draft killed; thin CMS-publish-check killed; emerging-topics duplicate killed).
- **0 stubs.** Everything in the manifest is shipping scope.
