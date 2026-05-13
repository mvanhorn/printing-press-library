# SurgeGraph CLI

**Every SurgeGraph workflow — research, write, publish, monitor — from the terminal or any agent, with a local SQLite that compounds week-over-week deltas the web app can't produce.**

SurgeGraph is an AI-search content operations cockpit. The web app shows point-in-time snapshots; this CLI keeps a local cache so you can run `visibility delta`, `prompts losers`, and `citation-domains rank-shift` over time. It compounds three split workflows — topic research, bulk writing, WordPress publishing — into a single `research gaps publish` pipeline, and ships every command as both CLI and MCP so the same agent that drafts an article can monitor the citations it generates.

Printed by [@ng-plentisoft](https://github.com/ng-plentisoft) (SurgeGraph Team).

## Install

The recommended path installs both the `surgegraph-pp-cli` binary and the `pp-surgegraph` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install surgegraph
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install surgegraph --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/surgegraph-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-surgegraph --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-surgegraph --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-surgegraph skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-surgegraph. The skill defines how its required CLI can be installed.
```

## Authentication

SurgeGraph uses OAuth 2.1 against https://mcp.surgegraph.io with Authorization Code + PKCE + Dynamic Client Registration. Run `surgegraph-pp-cli auth login` once; the CLI registers a client, walks you through the browser flow, and stores the bearer plus refresh token for subsequent calls. `auth status` shows the current TTL and `auth logout` clears it.

## Quick Start

```bash
# OAuth 2.1 + PKCE handshake against mcp.surgegraph.io; tokens cached locally.
surgegraph-pp-cli auth login


# Confirm bearer, quota, and tool reachability before doing anything else.
surgegraph-pp-cli doctor


# Pick a project_id; every visibility/docs/knowledge command needs one.
surgegraph-pp-cli get-projects --agent


# Populate the local store so deltas and search work.
surgegraph-pp-cli sync --project proj_abc123


# Week-over-week AI Visibility movers — the canonical Monday ritual.
surgegraph-pp-cli visibility delta --project proj_abc123 --window 168h --agent


# Preview the gap-to-WordPress pipeline before letting it actually post.
surgegraph-pp-cli research gaps publish --research-id res_xyz789 --project proj_abc123 --integration wp_int_456 --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`visibility delta`** — See exactly what moved in AI Visibility week over week — overview, trend, sentiment, or traffic — without screenshotting two dashboard tabs and diffing them by hand.

  _Use this when the user asks what changed in AI Visibility or wants a movers report; it answers without burning a follow-up API roundtrip._

  ```bash
  surgegraph-pp-cli visibility delta --project proj_abc123 --window 168h --metric overview,trend --agent
  ```
- **`visibility prompts losers`** — Surface tracked prompts whose citation count or first-position rank dropped vs the prior window — the report a CMO asks for every Monday and the dashboard can't produce.

  _Pick this when the user is hunting for visibility regressions; gives the agent a ranked, actionable list instead of two raw snapshots to compare._

  ```bash
  surgegraph-pp-cli visibility prompts losers --project proj_abc123 --since 720h --agent --select prompt_id,prompt,citation_delta,position_delta
  ```
- **`visibility citation-domains rank-shift`** — Watch which domains gained or lost citation share for your project's AI-tracked prompts over a 30-day window — useful when a competitor starts crowding the answer set.

  _Use when monitoring competitive share-of-voice in AI answers._

  ```bash
  surgegraph-pp-cli visibility citation-domains rank-shift --project proj_abc123 --window 720h --agent
  ```
- **`docs stale`** — List writer and optimized docs not updated in the last N days, ranked by recent AI-source traffic — the refresh queue in one query.

  _Use when prioritizing a content refresh queue._

  ```bash
  surgegraph-pp-cli docs stale --project proj_abc123 --older-than 90 --agent --select document_id,title,last_updated,ai_traffic_30d
  ```
- **`visibility portfolio`** — Roll up AI Visibility deltas across every project in the organization — the agency-portfolio view the single-project UI doesn't produce.

  _Use when the user manages multiple projects (agency, multi-brand) and asks for an executive summary._

  ```bash
  surgegraph-pp-cli visibility portfolio --window 168h --agent
  ```
- **`account burn`** — Project credit exhaustion from your last 30 days of usage snapshots — a pre-flight check before queueing a large bulk-publish run.

  _Pick this before kicking off a bulk operation to confirm you have credits._

  ```bash
  surgegraph-pp-cli account burn --window 720h --agent
  ```

### Cross-entity joins no single API call returns

- **`knowledge impact`** — Rank a project's knowledge libraries by how often their grounded URLs actually appear in tracked AI citations — answers "is this library worth maintaining?".

  _Pick this when the user audits knowledge libraries or asks which library is paying off._

  ```bash
  surgegraph-pp-cli knowledge impact --project proj_abc123 --agent
  ```
- **`research drift`** — Diff a topic_research's hierarchical map against the writer_documents you've already shipped — covered vs gap, ranked for action.

  _Use when planning a content sprint from a topic research; tells the agent exactly which subtopics still need articles._

  ```bash
  surgegraph-pp-cli research drift --research-id res_xyz789 --project proj_abc123 --agent --select topic,parent,covered
  ```
- **`research domain diff`** — Set-diff a competitor's domain_research topic tree against your own topic_research — instant view of what they cover and you don't.

  _Pick this when comparing topical coverage against a named competitor._

  ```bash
  surgegraph-pp-cli research domain diff --project proj_abc123 --mine res_xyz789 --theirs dom_res_aaa --agent
  ```
- **`visibility traffic-citations`** — For each top-traffic page, list the tracked prompts whose AI citations resolved to it — answers "why is this page getting AI traffic?".

  _Use when explaining traffic patterns or planning new content based on what's already pulling AI clicks._

  ```bash
  surgegraph-pp-cli visibility traffic-citations --project proj_abc123 --agent --select page_url,ai_visits,citations
  ```

### Compound workflows that span products

- **`research gaps publish`** — Take topic-gap output, bulk-create documents in 50-doc batches, and queue them into a WordPress integration — one idempotent command for the content-ops ritual.

  _Pick this when the user wants to ship articles for every gap from a topic research; collapses a multi-tab manual workflow into one call._

  ```bash
  surgegraph-pp-cli research gaps publish --research-id res_xyz789 --project proj_abc123 --integration wp_int_456 --dry-run --agent
  ```

### Agent-native plumbing

- **`context bundle`** — Emit a JSON blob shaped for an agent's context window covering prompts, citations, docs, and topic-maps for one project — the handoff payload an agent's loop needs.

  _Use to seed another agent's working memory with everything you know about a SurgeGraph project, in one round-trip._

  ```bash
  surgegraph-pp-cli context bundle --project proj_abc123 --include prompts,citations,docs,topics
  ```
- **`search`** — Full-text search across local cache: AI Visibility prompts, citations, writer documents, optimized documents, and topic-map nodes — one query, multi-entity hits.

  _Pick this when an agent needs to locate any signal — a prompt, a citation, a draft — across the whole local cache._

  ```bash
  surgegraph-pp-cli search "AI search optimization" --kind prompts,citations,docs,topics --agent --select kind,id,title,snippet
  ```
- **`sync diff`** — Emit per-resource sync cursors and total row counts from the local store — the inspection primitive for agent loops that need to know whether a re-sync is due.

  _Run after `sync` to inspect cursor freshness, or in an agent loop to decide whether the local store needs to be re-synced before reading._

  ```bash
  surgegraph-pp-cli sync diff --agent
  ```

## Usage

Run `surgegraph-pp-cli --help` for the full command reference and flag list.

## Commands

### create-ai-visibility-prompt

Manage create ai visibility prompt

- **`surgegraph-pp-cli create-ai-visibility-prompt create_ai_visibility_prompt`** - Create a new prompt to AI Visibility tracking. Prompts are tracked across AI answer engines to monitor brand visibility. Topics and tags are resolved by name and auto-created if they do not exist. Note: tag names are lowercased on write and lookup is case-insensitive — "My-Tag" and "my-tag" resolve to the same tag.

### create-api-key

Manage create api key

- **`surgegraph-pp-cli create-api-key create_api_key`** - Create an LLM provider API key for the organization. The key is validated against the provider before being stored. Optionally assign it to one or more projects. Note: each project can only be assigned to one API key per provider — assigning to a project that already has a key of the same provider will fail with a conflict.

### create-bulk-documents

Manage create bulk documents

- **`surgegraph-pp-cli create-bulk-documents create_bulk_documents`** - Bulk create multiple AI-written documents in a single batch (max 50). Each article needs a prompt; all other settings are shared.
Only 'general' and 'listicle' article types are supported for bulk generation. Requires a BYOK API key assigned to the project for the chosen model's provider (e.g. OpenAI BYOK for gpt-* models, Gemini BYOK for gemini-* models, Anthropic BYOK for claude-* models).
Articles are generated asynchronously with 30-second staggered delays between each.

IMPORTANT — webhook setup is REQUIRED for OpenAI bulk generation. Before calling this tool with an OpenAI (gpt-*) model, you MUST first ask the user to confirm they have added SurgeGraph's webhook URL "https://webhook-v2-935801613060.us-central1.run.app/openai" to their OpenAI organization webhook settings (https://platform.openai.com/settings/organization/webhooks). Without this webhook, OpenAI cannot deliver the generated articles back to SurgeGraph and the batch will silently never complete. Set 'webhookAcknowledged: true' ONLY after the user explicitly confirms; if they have not, instruct them to add the webhook first and do not proceed.

### create-document

Manage create document

- **`surgegraph-pp-cli create-document create_document`** - Create and generate an AI-written document. Only "prompt" is required — all other fields have sensible defaults.
Use get_writer_models to see available models, get_locations for location codes, and get_languages for language codes.
The document will be generated asynchronously. Use get_document to check its status.

Article-type-specific fields:
- articleType="review": pass `review.productName` (required) and optional `review.productUrl`.
- articleType="comparison": pass `comparison.custom` with at least 2 entries that have `productName`.
- articleType="roundup": pass `roundup.type` (serp | amazon | custom). For "amazon" you may pass `roundup.amazonProductSort`. For "custom" pass `roundup.custom`.
- articleType="listicle": optionally pass `listicle.type` (serp | custom). For "custom" pass `listicle.custom` as a newline-separated list of items.
Product URLs must contain http:// or https://.

### create-domain-research

Manage create domain research

- **`surgegraph-pp-cli create-domain-research create_domain_research`** - Start a domain research for topic coverage analysis. Analyzes a domain to discover relevant topics. Results are cached for 30 days.
Returns a research ID. The research runs asynchronously — use get_domain_research to check progress and fetch extracted topics, or list_domain_researches to see all entries.

### create-image

Manage create image

- **`surgegraph-pp-cli create-image create_image`** - Generate an AI image and store it in the Content Vision gallery. Use get_content_vision_settings first to see available image types and styles for the project.
Returns the generated image URL and metadata.

### create-knowledge-library

Manage create knowledge library

- **`surgegraph-pp-cli create-knowledge-library create_knowledge_library`** - Create a new knowledge library in a project. Knowledge libraries store documents that can be used as context for AI writing.

### create-knowledge-library-document

Manage create knowledge library document

- **`surgegraph-pp-cli create-knowledge-library-document create_knowledge_library_document`** - Create a text document to a knowledge library. The document content will be indexed for use as AI writing context. Max 40,000 words. Defaults to charging the indexing surcharge in credits; set creditChoice="byok" to skip the surcharge using an OpenAI key assigned to the project.

### create-optimized-document

Manage create optimized document

- **`surgegraph-pp-cli create-optimized-document create_optimized_document`** - Create a content-optimized document from an existing WordPress post. The system fetches the post content via wpPostId, then analyzes and rewrites it for better AEO/SEO performance.
Requires a WordPress integration to be configured for the project. The URL and title can be auto-resolved from the WordPress post if not provided.
Use get_writer_models for available models and get_locations/get_languages for codes.

### create-topic-research

Manage create topic research

- **`surgegraph-pp-cli create-topic-research create_topic_research`** - Start a topic research for topic coverage analysis. Analyzes a specific topic to discover subtopics, related keywords, and content gaps. Results are cached for 30 days.
Returns a research ID. Use list_topic_researches to see all researches and get_topic_research to fetch a specific result (use get_topic_map for the project-wide coverage map).
Use get_locations for location codes and get_languages for language codes.

### create-topic-research-expansion

Manage create topic research expansion

- **`surgegraph-pp-cli create-topic-research-expansion create_topic_research_expansion`** - Expand an existing topic research by generating micro topics under one or more macro topics. Each macro topic generates its own batch of micro topics in the background.
Consumes TOPIC_RESEARCH_EXPANSION credits — one credit per macro ID that is newly expanded (macro IDs already expanded are skipped and do not consume credit).
Use get_topic_research with a researchId to list macro topic IDs (nodes where type === "macro"). Results stream in asynchronously — poll get_topic_research to see new micro topics appear under each macro.

### delete-ai-visibility-prompt

Manage delete ai visibility prompt

- **`surgegraph-pp-cli delete-ai-visibility-prompt delete_ai_visibility_prompt`** - Delete an AI Visibility prompt from a project. Use get_ai_visibility_prompts to find prompt IDs. Useful for cleaning up over-quota state on the AEO_PROMPTS feature. This is a hard delete and cannot be undone.

### delete-api-key

Manage delete api key

- **`surgegraph-pp-cli delete-api-key delete_api_key`** - Delete an API key from the organization. Use get_openai_keys / get_gemini_keys / get_anthropic_keys to find the numeric key ID. Project assignments are removed automatically.

### delete-document

Manage delete document

- **`surgegraph-pp-cli delete-document delete_document`** - Delete a SurgeGraph document by ID. Use get_writer_documents or get_optimized_documents to find document IDs. Defaults to "trash" (soft delete) — the document is hidden from list/optimizer views but recoverable by setting status back. Pass mode="permanent" to hard-delete the row irrecoverably.

### delete-knowledge-library

Manage delete knowledge library

- **`surgegraph-pp-cli delete-knowledge-library delete_knowledge_library`** - Delete a knowledge library and all its documents. Use get_knowledge_libraries to find library IDs. This is a hard delete and cannot be undone. Decrements the KNOWLEDGE_LIBRARY usage counter.

### delete-knowledge-library-document

Manage delete knowledge library document

- **`surgegraph-pp-cli delete-knowledge-library-document delete_knowledge_library_document`** - Delete a document from a knowledge library. Removes the LlamaIndex indices, vector embeddings, and the document row. Use get_knowledge_library_documents to find document IDs. This is a hard delete and cannot be undone. Decrements the KNOWLEDGE_LIBRARY_ASSETS usage counter.

### get-ai-visibility-citation-domain

Manage get ai visibility citation domain

- **`surgegraph-pp-cli get-ai-visibility-citation-domain get_ai_visibility_citation_domain`** - Drill into citations for a specific domain. Returns page-level URLs cited from that domain, which engines cited each page, and which prompts triggered the citation per engine. Use get_ai_visibility_citations first to identify domains of interest.

### get-ai-visibility-citation-own-domain

Manage get ai visibility citation own domain

- **`surgegraph-pp-cli get-ai-visibility-citation-own-domain get_ai_visibility_citation_own_domain`** - Retrieve citation data for the project's own domain. Shows which of the user's pages are being cited by AI engines, which prompts triggered those citations per engine. The domain is auto-detected from the project's settings.

### get-ai-visibility-citations

Manage get ai visibility citations

- **`surgegraph-pp-cli get-ai-visibility-citations get_ai_visibility_citations`** - Retrieve high-level citation overview for AI Visibility tracking. Returns own-domain citation summary, per-engine diversification index, citation type breakdown, top cited domains, and citation overlap between engines. Use get_ai_visibility_citation_domain or get_ai_visibility_citation_own_domain for page-level drill-down.

### get-ai-visibility-config

Manage get ai visibility config

- **`surgegraph-pp-cli get-ai-visibility-config get_ai_visibility_config`** - Retrieve AI Visibility tracking and answer engine configuration for a project.

### get-ai-visibility-emerging-topics

Manage get ai visibility emerging topics

- **`surgegraph-pp-cli get-ai-visibility-emerging-topics get_ai_visibility_emerging_topics`** - Retrieve emerging topics from AI Visibility tracking. Returns topics whose mention frequency is rising across AI engine responses over the requested window, ranked from most to least emerging. Each entry exposes the topic name, growth rate, total frequency, emergence score, and opportunity score.

### get-ai-visibility-metadata

Manage get ai visibility metadata

- **`surgegraph-pp-cli get-ai-visibility-metadata get_ai_visibility_metadata`** - Retrieve filter metadata for an AI Visibility project. Returns the list of tracked answer engines (models), tracked brands, tracked engine providers, and total prompts monitored. Use this tool first to discover available modelIds and brandNames before calling other AI Visibility tools that accept filter parameters.

### get-ai-visibility-opportunities

Manage get ai visibility opportunities

- **`surgegraph-pp-cli get-ai-visibility-opportunities get_ai_visibility_opportunities`** - Retrieve AI Visibility optimization opportunities for a project. Returns stats by priority and category, plus a list of opportunities with their recommended actions. Categories: visibilityGaps, expansion, competitorIntel.

### get-ai-visibility-overview

Manage get ai visibility overview

- **`surgegraph-pp-cli get-ai-visibility-overview get_ai_visibility_overview`** - Retrieve AI Visibility overview for a specific brand in a project. Returns the brand's per-engine metrics (visibility, share of voice, position, mentions, prominence, sentiment, position distribution) and a competitive summary with the top 5 competitors.

### get-ai-visibility-prompt-detail

Manage get ai visibility prompt detail

- **`surgegraph-pp-cli get-ai-visibility-prompt-detail get_ai_visibility_prompt_detail`** - Deep dive into a single prompt on a specific date. Returns full brand performance (visibility, position, share, sentiment per engine), response structure, and citation details across all answer engines. Use get_ai_visibility_prompts to get prompt IDs first. Use promptExecutionId from engineResponses with get_ai_visibility_prompt_response to see the raw AI response.

### get-ai-visibility-prompt-response

Manage get ai visibility prompt response

- **`surgegraph-pp-cli get-ai-visibility-prompt-response get_ai_visibility_prompt_response`** - Retrieve the raw AI response for a specific prompt execution. Returns the structured response items and markdown content the answer engine produced. Use the promptExecutionId from get_ai_visibility_prompt_detail engineResponses field.

### get-ai-visibility-prompts

Manage get ai visibility prompts

- **`surgegraph-pp-cli get-ai-visibility-prompts get_ai_visibility_prompts`** - Retrieve prompts configured for AI Visibility tracking in a project. Paginated: defaults to page 1 with pageSize 50 (max 200). Provide startDate + endDate to attach averageVisibility and shareOfVoice for the project's tracked brand. Use sortBy="averageVisibility" with sortOrder="asc" to surface the worst-performing prompts. Returns a `{ data, meta }` envelope.

### get-ai-visibility-response-structure

Manage get ai visibility response structure

- **`surgegraph-pp-cli get-ai-visibility-response-structure get_ai_visibility_response_structure`** - Retrieve response structure analysis for a specific brand in AI Visibility tracking. Returns per-engine response format distribution (list, paragraphs, step_by_step, table, etc.), most common structure, average response length, and the brand's visibility within those structures. Also includes top competitors by visibility.

### get-ai-visibility-sentiment

Manage get ai visibility sentiment

- **`surgegraph-pp-cli get-ai-visibility-sentiment get_ai_visibility_sentiment`** - Retrieve sentiment analysis for a specific brand in AI Visibility tracking. Returns per-engine sentiment distribution, top positive/negative mentions, intent/aspect/tone breakdowns, and topical sentiment. Also includes top 5 most positively and negatively perceived competitors.

### get-ai-visibility-topic-gaps

Manage get ai visibility topic gaps

- **`surgegraph-pp-cli get-ai-visibility-topic-gaps get_ai_visibility_topic_gaps`** - Retrieve topic gaps for AI Visibility tracking. Topic gaps are topics where competitors are mentioned but your brand is not. Returns the missing topic, which competitors cover it, context clues, and the prompt that triggered it.

### get-ai-visibility-topics

Manage get ai visibility topics

- **`surgegraph-pp-cli get-ai-visibility-topics get_ai_visibility_topics`** - Retrieve topic clusters from AI Visibility tracking. Returns clustered topics with keyword frequencies, trend direction, and brand coverage. Optionally filter by brand to see association strength and top competitors per cluster.

### get-ai-visibility-traffic-pages

Manage get ai visibility traffic pages

- **`surgegraph-pp-cli get-ai-visibility-traffic-pages get_ai_visibility_traffic_pages`** - Retrieve paginated list of pages with AI traffic data. Returns per-page AI/human visits, CTR, crawl errors, top platform, and last indexed date. Supports search, sorting, and pagination.

### get-ai-visibility-traffic-summary

Manage get ai visibility traffic summary

- **`surgegraph-pp-cli get-ai-visibility-traffic-summary get_ai_visibility_traffic_summary`** - Retrieve AI traffic analytics summary for a project. Returns total AI/human visits with trends, CTR, indexed pages, crawl errors, top platform, and per-platform breakdown.

### get-ai-visibility-trend

Manage get ai visibility trend

- **`surgegraph-pp-cli get-ai-visibility-trend get_ai_visibility_trend`** - Retrieve daily AI Visibility trend for a specific brand. The date range must not exceed 14 days — the tool will reject requests with a larger range. Returns one data point per day with visibility, share of voice, position, and mentions. If no engine is specified, metrics are averaged across all engines.

### get-anthropic-keys

Manage get anthropic keys

- **`surgegraph-pp-cli get-anthropic-keys get_anthropic_keys`** - List all Anthropic API keys for the organization. Returns masked keys (last 4 chars visible), active status, label, and associated project IDs.

### get-author-brand

Manage get author brand

- **`surgegraph-pp-cli get-author-brand get_author_brand`** - Get the brand profile for a project (Author Synthesis). Returns brand identity values, mandatory/prohibited vocabulary, and target audience segments.

### get-authors

Manage get authors

- **`surgegraph-pp-cli get-authors get_authors`** - List all authors configured for a project (Author Synthesis). Returns author profiles with voice calibration, bio, writing corpus, and article counts.

### get-brand-mentions

Manage get brand mentions

- **`surgegraph-pp-cli get-brand-mentions get_brand_mentions`** - List all brand mentions configured for the organization.

### get-content-vision-gallery

Manage get content vision gallery

- **`surgegraph-pp-cli get-content-vision-gallery get_content_vision_gallery`** - List images from the Content Vision gallery for a project. Supports filtering by type, style, search query, and pagination.

### get-content-vision-settings

Manage get content vision settings

- **`surgegraph-pp-cli get-content-vision-settings get_content_vision_settings`** - Get Content Vision settings for a project including brand colors, featured image settings, and available image types with styles.

### get-document

Manage get document

- **`surgegraph-pp-cli get-document get_document`** - Retrieve a single document by ID, including its content, AEO (AI Engine Optimization) suggestions, and SEO suggestions.

### get-domain-research

Manage get domain research

- **`surgegraph-pp-cli get-domain-research get_domain_research`** - Get the full result of a domain research: the domain, extracted topics, and completion status. Use list_domain_researches to find research IDs.

### get-gemini-keys

Manage get gemini keys

- **`surgegraph-pp-cli get-gemini-keys get_gemini_keys`** - List all Gemini API keys for the organization. Returns masked keys (last 4 chars visible), active status, label, and associated project IDs.

### get-knowledge-libraries

Manage get knowledge libraries

- **`surgegraph-pp-cli get-knowledge-libraries get_knowledge_libraries`** - List all knowledge libraries for a project. Returns libraries with document and chat counts.

### get-knowledge-library-documents

Manage get knowledge library documents

- **`surgegraph-pp-cli get-knowledge-library-documents get_knowledge_library_documents`** - List all documents/assets in a knowledge library. Returns document details including indexing status.

### get-languages

Manage get languages

- **`surgegraph-pp-cli get-languages get_languages`** - List available languages for document creation and topic research. Returns language code and name. Pass the returned code verbatim to any tool that accepts languageCode (e.g. create_document, create_topic_research).

### get-locations

Manage get locations

- **`surgegraph-pp-cli get-locations get_locations`** - List available locations (countries) for document creation and topic research. Returns location code, name, and country ISO code. Use the location code when creating documents.

### get-openai-keys

Manage get openai keys

- **`surgegraph-pp-cli get-openai-keys get_openai_keys`** - List all OpenAI API keys for the organization. Returns masked keys (last 4 chars visible), active status, label, and associated project IDs.

### get-optimized-documents

Manage get optimized documents

- **`surgegraph-pp-cli get-optimized-documents get_optimized_documents`** - List optimized articles from Content Optimizer. Supports search and pagination.

### get-organization-cms-integrations

Manage get organization cms integrations

- **`surgegraph-pp-cli get-organization-cms-integrations get_organization_cms_integrations`** - List all CMS integrations connected to the organization, including which projects each integration is linked to. Shows WordPress site details and connection status.

### get-project-cms-integration

Manage get project cms integration

- **`surgegraph-pp-cli get-project-cms-integration get_project_cms_integration`** - Get the CMS integration connected to a specific project. Returns the integration type, site details, and connection status.

### get-projects

Manage get projects

- **`surgegraph-pp-cli get-projects get_projects`** - List all projects for the organization. Supports search and pagination.

### get-team

Manage get team

- **`surgegraph-pp-cli get-team get_team`** - List all team members and pending invitations for the organization. Returns member details including roles and project access.

### get-topic-map

Manage get topic map

- **`surgegraph-pp-cli get-topic-map get_topic_map`** - Get the topic coverage map for a project. Returns the full topic hierarchy with pillar topics, subtopics, coverage status, priority scores, and progress statistics.

### get-topic-research

Manage get topic research

- **`surgegraph-pp-cli get-topic-research get_topic_research`** - Get the full result of a topic research: seed topic, location/language, and the hierarchical macro → micro topic tree with keyword data. Use list_topic_researches to find research IDs, or use create_topic_research_expansion to generate more micro topics.

### get-usage

Manage get usage

- **`surgegraph-pp-cli get-usage get_usage`** - Get quota usage, feature limits, and credit balance for the organization. Shows permanent quotas (team seats, projects, etc.), cycle quotas (documents), and available credits.

### get-wordpress-authors

Manage get wordpress authors

- **`surgegraph-pp-cli get-wordpress-authors get_wordpress_authors`** - List authors from a connected WordPress site. Returns id, name, slug, bio, and avatarUrl per author. Paginated — use page/perPage. Pass an id to publish_document_to_cms → authorId to override the document's mapped author.

### get-wordpress-categories

Manage get wordpress categories

- **`surgegraph-pp-cli get-wordpress-categories get_wordpress_categories`** - List all categories from a connected WordPress site. Returns id, name, and slug for each category — use the ids with publish_document_to_cms → categoryIds.

### get-wordpress-integrations

Manage get wordpress integrations

- **`surgegraph-pp-cli get-wordpress-integrations get_wordpress_integrations`** - List all WordPress integrations connected to the organization. Shows site details, masked API keys, and which projects each integration is linked to.

### get-writer-documents

Manage get writer documents

- **`surgegraph-pp-cli get-writer-documents get_writer_documents`** - List articles from Content Hub. Supports filters and pagination.

### get-writer-models

Manage get writer models

- **`surgegraph-pp-cli get-writer-models get_writer_models`** - List available AI models for document writing. Returns model IDs, providers, credit costs, and whether they support thinking mode. Use the model ID in the "model" field when creating documents.

### list-domain-researches

Manage list domain researches

- **`surgegraph-pp-cli list-domain-researches list_domain_researches`** - List all domain researches for a project. Returns each entry with its id, domain, extracted topic count, and timestamps — newest first. Use get_domain_research to fetch the full topic list for a specific entry.

### list-topic-researches

Manage list topic researches

- **`surgegraph-pp-cli list-topic-researches list_topic_researches`** - List all topic researches for a project. Returns each entry with its id, seed topic, total topic count, and timestamps — newest first. Use get_topic_research with a specific ID to fetch the full hierarchical result.

### publish-document-to-cms

Manage publish document to cms

- **`surgegraph-pp-cli publish-document-to-cms publish_document_to_cms`** - Publish a SurgeGraph document to the CMS connected to its project (currently WordPress). Creates a new post on the target site with the document's title, content, meta description, schema markup, featured image, and inline images. Product shortcodes are enriched with product metadata before publish.
The target integration is derived from the document's project. If the project has no CMS integration connected, this tool fails — call update_project_cms_integration first to connect one. Use get_wordpress_categories to look up categoryIds, get_wordpress_authors to look up authorId. If authorId is omitted, the document's mapped WordPress author is used.

### update-ai-visibility-prompt

Manage update ai visibility prompt

- **`surgegraph-pp-cli update-ai-visibility-prompt update_ai_visibility_prompt`** - Update an existing AI Visibility prompt. Can modify the prompt text, topic, and tags. Topics and tags are resolved by name and auto-created if they do not exist. Use get_ai_visibility_prompts to get prompt IDs. Note: tag names are lowercased on write and lookup is case-insensitive — "My-Tag" and "my-tag" resolve to the same tag.

### update-api-key

Manage update api key

- **`surgegraph-pp-cli update-api-key update_api_key`** - Update an existing API key. Can change the raw key, active status, label, or project assignments. Passing `projectIds` replaces the full list of assigned projects (pass `[]` to unassign all). Each project can only be assigned to one key per provider — conflicts throw an error. Use get_openai_keys / get_gemini_keys / get_anthropic_keys to find the numeric key ID.

### update-document

Manage update document

- **`surgegraph-pp-cli update-document update_document`** - Update an existing SurgeGraph document's basic fields: title, content (HTML body), meta description, and/or schema markup. Records a history snapshot automatically when title or content changes. For publishing to WordPress use publish_document_to_cms.
Scope is intentionally narrow — outline, author, writing settings, and image generation are not editable via this tool. Use get_document to inspect the current state before editing. When 'mode' is 'append', the provided content is concatenated to the existing body instead of replacing it.

### update-project-cms-integration

Manage update project cms integration

- **`surgegraph-pp-cli update-project-cms-integration update_project_cms_integration`** - Connect or change the CMS integration for a project (currently WordPress only). Use get_wordpress_integrations to find the integration ID. After this is set, publish_document_to_cms will publish documents from this project to that integration. Replaces any existing connection on the project.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
surgegraph-pp-cli create-ai-visibility-prompt --intent example-value

# JSON for scripting and agents
surgegraph-pp-cli create-ai-visibility-prompt --intent example-value --json

# Filter to specific fields
surgegraph-pp-cli create-ai-visibility-prompt --intent example-value --json --select id,name,status

# Dry run — show the request without sending
surgegraph-pp-cli create-ai-visibility-prompt --intent example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
surgegraph-pp-cli create-ai-visibility-prompt --intent example-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-surgegraph -g
```

Then invoke `/pp-surgegraph <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add surgegraph surgegraph-pp-mcp -e SURGEGRAPH_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/surgegraph-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SURGEGRAPH_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "surgegraph": {
      "command": "surgegraph-pp-mcp",
      "env": {
        "SURGEGRAPH_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
surgegraph-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/surgegraph-facade-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SURGEGRAPH_TOKEN` | harvested | Yes | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `surgegraph-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SURGEGRAPH_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any command** — Run `surgegraph-pp-cli auth login`. If you were authed, the refresh token expired — `auth status` shows the TTL.
- **Empty `visibility delta` output** — Run `surgegraph-pp-cli sync --project <id>` at least twice on different days; deltas need two snapshots.
- **`visibility prompts losers` lists no prompts** — Verify with `visibility prompts list --project <id>` that the project tracks any prompts at all. Empty losers list with non-empty prompts means nothing dropped — that's the answer.
- **`research gaps publish` errors out partway through** — It's idempotent on the bulk-create step. Re-run with `--dry-run` first; matching documents are skipped by external_id.
- **Rate-limit errors during bulk operations** — The CLI uses cliutil.AdaptiveLimiter and backs off automatically. If errors persist after retry, run `account burn --window 168h` — you may be near the quota ceiling.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
