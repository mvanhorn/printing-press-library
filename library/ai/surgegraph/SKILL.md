---
name: pp-surgegraph
description: "Every SurgeGraph workflow — research, write, publish, monitor — from the terminal or any agent, with a local... Trigger phrases: `what changed in SurgeGraph this week`, `AI Visibility for my project`, `which prompts lost AI citations`, `publish gap articles from a topic research`, `what knowledge libraries actually show up in citations`, `use surgegraph`, `run surgegraph`."
author: "SurgeGraph Team"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - surgegraph-pp-cli
---

# SurgeGraph — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `surgegraph-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install surgegraph --cli-only
   ```
2. Verify: `surgegraph-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

SurgeGraph is an AI-search content operations cockpit. The web app shows point-in-time snapshots; this CLI keeps a local cache so you can run `visibility delta`, `prompts losers`, and `citation-domains rank-shift` over time. It compounds three split workflows — topic research, bulk writing, WordPress publishing — into a single `research gaps publish` pipeline, and ships every command as both CLI and MCP so the same agent that drafts an article can monitor the citations it generates.

## When to Use This CLI

Reach for this CLI when an agent needs to operate against SurgeGraph for any AI-search content task: monitoring AI Visibility deltas, drafting and publishing articles in bulk, auditing knowledge-library impact, or seeding another agent with a project's current state. Every visible web-app feature is also a CLI command, and every transcendence command is an MCP tool — the agent that opened the loop can close it without leaving the terminal.

## Unique Capabilities

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

## Command Reference

**create-ai-visibility-prompt** — Manage create ai visibility prompt

- `surgegraph-pp-cli create-ai-visibility-prompt` — Create a new prompt to AI Visibility tracking. Prompts are tracked across AI answer engines to monitor brand...

**create-api-key** — Manage create api key

- `surgegraph-pp-cli create-api-key` — Create an LLM provider API key for the organization. The key is validated against the provider before being stored....

**create-bulk-documents** — Manage create bulk documents

- `surgegraph-pp-cli create-bulk-documents` — Bulk create multiple AI-written documents in a single batch (max 50). Each article needs a prompt; all other...

**create-document** — Manage create document

- `surgegraph-pp-cli create-document` — Create and generate an AI-written document. Only 'prompt' is required — all other fields have sensible defaults....

**create-domain-research** — Manage create domain research

- `surgegraph-pp-cli create-domain-research` — Start a domain research for topic coverage analysis. Analyzes a domain to discover relevant topics. Results are...

**create-image** — Manage create image

- `surgegraph-pp-cli create-image` — Generate an AI image and store it in the Content Vision gallery. Use get_content_vision_settings first to see...

**create-knowledge-library** — Manage create knowledge library

- `surgegraph-pp-cli create-knowledge-library` — Create a new knowledge library in a project. Knowledge libraries store documents that can be used as context for AI...

**create-knowledge-library-document** — Manage create knowledge library document

- `surgegraph-pp-cli create-knowledge-library-document` — Create a text document to a knowledge library. The document content will be indexed for use as AI writing context....

**create-optimized-document** — Manage create optimized document

- `surgegraph-pp-cli create-optimized-document` — Create a content-optimized document from an existing WordPress post. The system fetches the post content via...

**create-topic-research** — Manage create topic research

- `surgegraph-pp-cli create-topic-research` — Start a topic research for topic coverage analysis. Analyzes a specific topic to discover subtopics, related...

**create-topic-research-expansion** — Manage create topic research expansion

- `surgegraph-pp-cli create-topic-research-expansion` — Expand an existing topic research by generating micro topics under one or more macro topics. Each macro topic...

**delete-ai-visibility-prompt** — Manage delete ai visibility prompt

- `surgegraph-pp-cli delete-ai-visibility-prompt` — Delete an AI Visibility prompt from a project. Use get_ai_visibility_prompts to find prompt IDs. Useful for cleaning...

**delete-api-key** — Manage delete api key

- `surgegraph-pp-cli delete-api-key` — Delete an API key from the organization. Use get_openai_keys / get_gemini_keys / get_anthropic_keys to find the...

**delete-document** — Manage delete document

- `surgegraph-pp-cli delete-document` — Delete a SurgeGraph document by ID. Use get_writer_documents or get_optimized_documents to find document IDs....

**delete-knowledge-library** — Manage delete knowledge library

- `surgegraph-pp-cli delete-knowledge-library` — Delete a knowledge library and all its documents. Use get_knowledge_libraries to find library IDs. This is a hard...

**delete-knowledge-library-document** — Manage delete knowledge library document

- `surgegraph-pp-cli delete-knowledge-library-document` — Delete a document from a knowledge library. Removes the LlamaIndex indices, vector embeddings, and the document row....

**get-ai-visibility-citation-domain** — Manage get ai visibility citation domain

- `surgegraph-pp-cli get-ai-visibility-citation-domain` — Drill into citations for a specific domain. Returns page-level URLs cited from that domain, which engines cited each...

**get-ai-visibility-citation-own-domain** — Manage get ai visibility citation own domain

- `surgegraph-pp-cli get-ai-visibility-citation-own-domain` — Retrieve citation data for the project's own domain. Shows which of the user's pages are being cited by AI engines,...

**get-ai-visibility-citations** — Manage get ai visibility citations

- `surgegraph-pp-cli get-ai-visibility-citations` — Retrieve high-level citation overview for AI Visibility tracking. Returns own-domain citation summary, per-engine...

**get-ai-visibility-config** — Manage get ai visibility config

- `surgegraph-pp-cli get-ai-visibility-config` — Retrieve AI Visibility tracking and answer engine configuration for a project.

**get-ai-visibility-emerging-topics** — Manage get ai visibility emerging topics

- `surgegraph-pp-cli get-ai-visibility-emerging-topics` — Retrieve emerging topics from AI Visibility tracking. Returns topics whose mention frequency is rising across AI...

**get-ai-visibility-metadata** — Manage get ai visibility metadata

- `surgegraph-pp-cli get-ai-visibility-metadata` — Retrieve filter metadata for an AI Visibility project. Returns the list of tracked answer engines (models), tracked...

**get-ai-visibility-opportunities** — Manage get ai visibility opportunities

- `surgegraph-pp-cli get-ai-visibility-opportunities` — Retrieve AI Visibility optimization opportunities for a project. Returns stats by priority and category, plus a list...

**get-ai-visibility-overview** — Manage get ai visibility overview

- `surgegraph-pp-cli get-ai-visibility-overview` — Retrieve AI Visibility overview for a specific brand in a project. Returns the brand's per-engine metrics...

**get-ai-visibility-prompt-detail** — Manage get ai visibility prompt detail

- `surgegraph-pp-cli get-ai-visibility-prompt-detail` — Deep dive into a single prompt on a specific date. Returns full brand performance (visibility, position, share,...

**get-ai-visibility-prompt-response** — Manage get ai visibility prompt response

- `surgegraph-pp-cli get-ai-visibility-prompt-response` — Retrieve the raw AI response for a specific prompt execution. Returns the structured response items and markdown...

**get-ai-visibility-prompts** — Manage get ai visibility prompts

- `surgegraph-pp-cli get-ai-visibility-prompts` — Retrieve prompts configured for AI Visibility tracking in a project. Paginated: defaults to page 1 with pageSize 50...

**get-ai-visibility-response-structure** — Manage get ai visibility response structure

- `surgegraph-pp-cli get-ai-visibility-response-structure` — Retrieve response structure analysis for a specific brand in AI Visibility tracking. Returns per-engine response...

**get-ai-visibility-sentiment** — Manage get ai visibility sentiment

- `surgegraph-pp-cli get-ai-visibility-sentiment` — Retrieve sentiment analysis for a specific brand in AI Visibility tracking. Returns per-engine sentiment...

**get-ai-visibility-topic-gaps** — Manage get ai visibility topic gaps

- `surgegraph-pp-cli get-ai-visibility-topic-gaps` — Retrieve topic gaps for AI Visibility tracking. Topic gaps are topics where competitors are mentioned but your brand...

**get-ai-visibility-topics** — Manage get ai visibility topics

- `surgegraph-pp-cli get-ai-visibility-topics` — Retrieve topic clusters from AI Visibility tracking. Returns clustered topics with keyword frequencies, trend...

**get-ai-visibility-traffic-pages** — Manage get ai visibility traffic pages

- `surgegraph-pp-cli get-ai-visibility-traffic-pages` — Retrieve paginated list of pages with AI traffic data. Returns per-page AI/human visits, CTR, crawl errors, top...

**get-ai-visibility-traffic-summary** — Manage get ai visibility traffic summary

- `surgegraph-pp-cli get-ai-visibility-traffic-summary` — Retrieve AI traffic analytics summary for a project. Returns total AI/human visits with trends, CTR, indexed pages,...

**get-ai-visibility-trend** — Manage get ai visibility trend

- `surgegraph-pp-cli get-ai-visibility-trend` — Retrieve daily AI Visibility trend for a specific brand. The date range must not exceed 14 days — the tool will...

**get-anthropic-keys** — Manage get anthropic keys

- `surgegraph-pp-cli get-anthropic-keys` — List all Anthropic API keys for the organization. Returns masked keys (last 4 chars visible), active status, label,...

**get-author-brand** — Manage get author brand

- `surgegraph-pp-cli get-author-brand` — Get the brand profile for a project (Author Synthesis). Returns brand identity values, mandatory/prohibited...

**get-authors** — Manage get authors

- `surgegraph-pp-cli get-authors` — List all authors configured for a project (Author Synthesis). Returns author profiles with voice calibration, bio,...

**get-brand-mentions** — Manage get brand mentions

- `surgegraph-pp-cli get-brand-mentions` — List all brand mentions configured for the organization.

**get-content-vision-gallery** — Manage get content vision gallery

- `surgegraph-pp-cli get-content-vision-gallery` — List images from the Content Vision gallery for a project. Supports filtering by type, style, search query, and...

**get-content-vision-settings** — Manage get content vision settings

- `surgegraph-pp-cli get-content-vision-settings` — Get Content Vision settings for a project including brand colors, featured image settings, and available image types...

**get-document** — Manage get document

- `surgegraph-pp-cli get-document` — Retrieve a single document by ID, including its content, AEO (AI Engine Optimization) suggestions, and SEO suggestions.

**get-domain-research** — Manage get domain research

- `surgegraph-pp-cli get-domain-research` — Get the full result of a domain research: the domain, extracted topics, and completion status. Use...

**get-gemini-keys** — Manage get gemini keys

- `surgegraph-pp-cli get-gemini-keys` — List all Gemini API keys for the organization. Returns masked keys (last 4 chars visible), active status, label, and...

**get-knowledge-libraries** — Manage get knowledge libraries

- `surgegraph-pp-cli get-knowledge-libraries` — List all knowledge libraries for a project. Returns libraries with document and chat counts.

**get-knowledge-library-documents** — Manage get knowledge library documents

- `surgegraph-pp-cli get-knowledge-library-documents` — List all documents/assets in a knowledge library. Returns document details including indexing status.

**get-languages** — Manage get languages

- `surgegraph-pp-cli get-languages` — List available languages for document creation and topic research. Returns language code and name. Pass the returned...

**get-locations** — Manage get locations

- `surgegraph-pp-cli get-locations` — List available locations (countries) for document creation and topic research. Returns location code, name, and...

**get-openai-keys** — Manage get openai keys

- `surgegraph-pp-cli get-openai-keys` — List all OpenAI API keys for the organization. Returns masked keys (last 4 chars visible), active status, label, and...

**get-optimized-documents** — Manage get optimized documents

- `surgegraph-pp-cli get-optimized-documents` — List optimized articles from Content Optimizer. Supports search and pagination.

**get-organization-cms-integrations** — Manage get organization cms integrations

- `surgegraph-pp-cli get-organization-cms-integrations` — List all CMS integrations connected to the organization, including which projects each integration is linked to....

**get-project-cms-integration** — Manage get project cms integration

- `surgegraph-pp-cli get-project-cms-integration` — Get the CMS integration connected to a specific project. Returns the integration type, site details, and connection...

**get-projects** — Manage get projects

- `surgegraph-pp-cli get-projects` — List all projects for the organization. Supports search and pagination.

**get-team** — Manage get team

- `surgegraph-pp-cli get-team` — List all team members and pending invitations for the organization. Returns member details including roles and...

**get-topic-map** — Manage get topic map

- `surgegraph-pp-cli get-topic-map` — Get the topic coverage map for a project. Returns the full topic hierarchy with pillar topics, subtopics, coverage...

**get-topic-research** — Manage get topic research

- `surgegraph-pp-cli get-topic-research` — Get the full result of a topic research: seed topic, location/language, and the hierarchical macro → micro topic...

**get-usage** — Manage get usage

- `surgegraph-pp-cli get-usage` — Get quota usage, feature limits, and credit balance for the organization. Shows permanent quotas (team seats,...

**get-wordpress-authors** — Manage get wordpress authors

- `surgegraph-pp-cli get-wordpress-authors` — List authors from a connected WordPress site. Returns id, name, slug, bio, and avatarUrl per author. Paginated —...

**get-wordpress-categories** — Manage get wordpress categories

- `surgegraph-pp-cli get-wordpress-categories` — List all categories from a connected WordPress site. Returns id, name, and slug for each category — use the ids...

**get-wordpress-integrations** — Manage get wordpress integrations

- `surgegraph-pp-cli get-wordpress-integrations` — List all WordPress integrations connected to the organization. Shows site details, masked API keys, and which...

**get-writer-documents** — Manage get writer documents

- `surgegraph-pp-cli get-writer-documents` — List articles from Content Hub. Supports filters and pagination.

**get-writer-models** — Manage get writer models

- `surgegraph-pp-cli get-writer-models` — List available AI models for document writing. Returns model IDs, providers, credit costs, and whether they support...

**list-domain-researches** — Manage list domain researches

- `surgegraph-pp-cli list-domain-researches` — List all domain researches for a project. Returns each entry with its id, domain, extracted topic count, and...

**list-topic-researches** — Manage list topic researches

- `surgegraph-pp-cli list-topic-researches` — List all topic researches for a project. Returns each entry with its id, seed topic, total topic count, and...

**publish-document-to-cms** — Manage publish document to cms

- `surgegraph-pp-cli publish-document-to-cms` — Publish a SurgeGraph document to the CMS connected to its project (currently WordPress). Creates a new post on the...

**update-ai-visibility-prompt** — Manage update ai visibility prompt

- `surgegraph-pp-cli update-ai-visibility-prompt` — Update an existing AI Visibility prompt. Can modify the prompt text, topic, and tags. Topics and tags are resolved...

**update-api-key** — Manage update api key

- `surgegraph-pp-cli update-api-key` — Update an existing API key. Can change the raw key, active status, label, or project assignments. Passing...

**update-document** — Manage update document

- `surgegraph-pp-cli update-document` — Update an existing SurgeGraph document's basic fields: title, content (HTML body), meta description, and/or schema...

**update-project-cms-integration** — Manage update project cms integration

- `surgegraph-pp-cli update-project-cms-integration` — Connect or change the CMS integration for a project (currently WordPress only). Use get_wordpress_integrations to...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
surgegraph-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monday AI Visibility review

```bash
surgegraph-pp-cli visibility delta --project proj_abc123 --window 168h --agent --select brand,metric,delta_pct
```

Pull week-over-week movers and pipe straight into the Monday CMO update.

### Trim a deeply-nested citations response

```bash
surgegraph-pp-cli get-ai-visibility-citations --project-id proj_abc123 --agent --select data.domains.domain,data.domains.citationCount
```

Citation responses are deeply nested; dotted-path `--select` keeps the payload under an agent's context budget.

### Find every signal about a topic across the local cache

```bash
surgegraph-pp-cli search "AI search optimization" --kind prompt,citation,doc,topic --agent --select kind,id,title,snippet
```

FTS5 across all four entities in one round-trip; results are unified and rankable. Run `sync` first to populate the index.

### Publish every gap topic to WordPress

```bash
surgegraph-pp-cli research gaps publish --research-id res_xyz789 --project proj_abc123 --integration wp_int_456 --dry-run
```

Compounds get_topic_map → create_bulk_documents → publish_document_to_cms; idempotent on retries.

### Filter results to one citation kind from the local cache

```bash
surgegraph-pp-cli search "answer engine" --kind citation --agent
```

Limit FTS hits to one entity kind via `--kind`; combine with `--limit` to keep output bounded.

## Auth Setup

SurgeGraph uses OAuth 2.1 against https://mcp.surgegraph.io with Authorization Code + PKCE + Dynamic Client Registration. Run `surgegraph-pp-cli auth login` once; the CLI registers a client, walks you through the browser flow, and stores the bearer plus refresh token for subsequent calls. `auth status` shows the current TTL and `auth logout` clears it.

Run `surgegraph-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  surgegraph-pp-cli create-ai-visibility-prompt --intent example-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
surgegraph-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
surgegraph-pp-cli feedback --stdin < notes.txt
surgegraph-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.surgegraph-pp-cli/feedback.jsonl`. They are never POSTed unless `SURGEGRAPH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SURGEGRAPH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
surgegraph-pp-cli profile save briefing --json
surgegraph-pp-cli --profile briefing create-ai-visibility-prompt --intent example-value
surgegraph-pp-cli profile list --json
surgegraph-pp-cli profile show briefing
surgegraph-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `surgegraph-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add surgegraph-pp-mcp -- surgegraph-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which surgegraph-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   surgegraph-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `surgegraph-pp-cli <command> --help`.
