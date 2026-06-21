# Valyu CLI

**Search the web, academic papers, financial data, and patents in one call — with offline history, cost tracking, and citation export.**

Valyu DeepSearch queries 40+ proprietary datasets (arXiv, PubMed, SEC filings, USPTO patents, financial reports) alongside the open web in a single API call. This CLI adds local history, A/B source comparison, batch queuing, and academic citation export — features the API itself does not surface.

## Products

| Product | Endpoint | Description |
| --- | --- | --- |
| **Search** | `POST /v1/search` | Multi-source search across web and proprietary datasets |
| **Contents** | `POST /v1/contents` | Extract clean, structured content from URLs |
| **Answer** | `POST /v1/answer` | AI-powered answers with real-time search |
| **DeepResearch** | `POST /v1/deepresearch/tasks` | Async research agents for comprehensive analysis |
| **Workflows** (Beta) | `GET /v1/workflows` | Templated, versioned DeepResearch for repeatable knowledge work |
| **Datasources** | `GET /v1/datasources` | Discover available data sources and their schemas |

## Authentication

Set VALYU_API_KEY from app.valyu.world. The CLI reads it automatically; run 'valyu-pp-cli auth set-token <key>' to persist it.

## Pricing

Valyu uses transparent, pay-per-use pricing. Credits are shared across every API.
- **Search**: CPM-based (cost per mille tokens retrieved)
- **Contents**: $0.001 per URL extracted, +$0.001 with AI features
- **Answer**: $0.10 per request + variable search and AI costs
- **DeepResearch**: Fixed per-task pricing by mode ($0.10 - $15.00)

All plans include web search and open academic sources. A subscription unlocks specialised and proprietary sources (SEC filings, patents, drug discovery, genomics, and more) at a lower cost per credit. See [docs.valyu.ai](https://docs.valyu.ai) for full pricing details.

## SDKs

- [Python SDK](https://pypi.org/project/valyu/) - `pip install valyu`
- [TypeScript SDK](https://www.npmjs.com/package/valyu) - `npm install valyu`

Learn more at [Valyu](https://valyu.ai).

Created by [@AnchalSharma19](https://github.com/AnchalSharma19) (Anchal Sharma).

## Install

The recommended path installs both the `valyu-pp-cli` binary and the `pp-valyu` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install valyu
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install valyu --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install valyu --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install valyu --agent claude-code
npx -y @mvanhorn/printing-press-library install valyu --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/valyu/cmd/valyu-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/valyu-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install valyu --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-valyu --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-valyu --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install valyu --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/valyu-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `VALYU_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/valyu/cmd/valyu-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "valyu": {
      "command": "valyu-pp-mcp",
      "env": {
        "VALYU_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the CLI is installed and auth key is configured
valyu-pp-cli doctor --dry-run

# Search academic papers — returns ranked results with source URLs
valyu-pp-cli semantic-search --query "CRISPR therapeutic applications 2024" --search-type paper --max-num-results 5 --agent

# Get a researched answer with exported citations
valyu-pp-cli cited-answer --query "What are the main risks of GLP-1 drugs?" --cite-output refs.json --agent

# Compare academic vs web sources for the same query
valyu-pp-cli semantic-search compare --query "AI regulation Europe" --config-a "type=paper" --config-b "type=web" --agent

# Check cumulative API spend tracked locally
valyu-pp-cli cost --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Search intelligence
- **`semantic-search compare`** — Run the same query against two source configurations and diff results, overlap, and cost side-by-side.

  _Use when tuning search configuration: see which source type finds unique vs overlapping results for your query._

  ```bash
  valyu-pp-cli semantic-search compare --query "CRISPR gene editing" --config-a "type=paper" --config-b "type=web" --agent
  ```
- **`semantic-search batch`** — Run multiple searches from a query file and emit JSONL results.

  _Use when you need results for many queries at once without writing a shell loop._

  ```bash
  valyu-pp-cli semantic-search batch --file queries.txt --agent --deliver file:/tmp/results.jsonl
  ```

### Local state that compounds
- **`cost`** — Show cumulative Valyu API spend tracked locally by this CLI.

  _Use to monitor spend before hitting budget limits, without logging into the Valyu dashboard._

  ```bash
  valyu-pp-cli cost --agent
  ```
- **`history`** — View past searches stored locally and re-run any with the original parameters.

  _Use to audit what was searched and reproduce past results without re-typing parameters._

  ```bash
  valyu-pp-cli history list --agent --limit 20
  ```
- **`semantic-search run`** — Save a named search configuration once, then run it on demand without re-specifying flags.

  _Use for recurring research workflows where the same query + source configuration is run repeatedly._

  ```bash
  valyu-pp-cli semantic-search run biotech-weekly --agent
  ```

### Agent-native plumbing
- **`contents pipe`** — Pipe search result URLs directly into content extraction in one step.

  _Use when you need full page text from search results without manually copying URLs._

  ```bash
  valyu-pp-cli semantic-search --query "AI regulation 2025" --agent | valyu-pp-cli contents pipe --from-stdin --agent
  ```
- **`cited-answer`** — Get a cited answer and export references as BibTeX, RIS, or JSON.

  _Use when you need a researched answer with properly formatted citations for academic or professional writing._

  ```bash
  valyu-pp-cli cited-answer --query "What caused the 2008 financial crisis?" --cite-output refs.bib --format bibtex --agent
  ```
- **`deepresearch queue`** — Submit a batch of async deep-research tasks and track their progress.

  _Use when running many parallel deep-research tasks; tracks completion without manual polling._

  ```bash
  valyu-pp-cli deepresearch queue --file topics.txt --agent
  ```

## Usage

Run `valyu-pp-cli --help` for the full command reference and flag list.

## Commands

### answer

Get AI-powered answers grounded in real-time search results. The Answer API searches across web, academic, and financial sources, then uses AI to generate a readable response via Server-Sent Events streaming.

- **`valyu-pp-cli answer`** - Searches across web, academic, and financial sources, then uses AI to generate a readable, cited response. Returns results as a **Server-Sent Events (SSE)** stream.

The AI agent performs multi-round search with up to 2 tool call rounds (3 in `fast_mode`), can decompose queries into sub-queries, and synthesizes findings into a coherent answer.

## SSE Event Types

Events are sent as `data: {json}\n\n` with the following types:

| Event | Description |
| --- | --- |
| `search_results` | Search sources found (may occur multiple times) |
| `content` | Partial answer text chunk (OpenAI-compatible delta format) |
| `metadata` | Final metadata with costs, token usage, and all search results |
| `[DONE]` | Stream complete |

## Cost Structure

Each request costs a base of ~$0.10 plus variable search and AI token costs. The `cost` object in the metadata event provides a full breakdown.

### contents

Extract clean, structured content from web pages at scale. Supports batch processing, AI-powered summaries, structured extraction via JSON schemas, and async jobs for large URL sets.

- **`valyu-pp-cli contents extract`** - Turn any web page into clean, structured data. Extract content from up to 10 URLs per request with optional AI-powered summaries, structured extraction via JSON schemas, and screenshot capture.

- **Synchronous (default)**: Returns extracted content directly
- **Async**: Set `async: true` to process in the background and poll a `job_id`
- **Pay-per-success**: Only charged for URLs that are successfully processed ($0.001/URL, +$0.001 with AI features). Failed URLs are free
- **Partial / total failure**: A `206` is returned when some URLs fail; a `422` is returned when every URL fails
- **`valyu-pp-cli contents get-job`** - Check the status and retrieve results of an async content extraction job.

### datasources

Discover available data sources and their schemas. A tool manifest for AI agents - instead of hardcoding knowledge of available datasets, agents can query this API to discover sources, filter by category, and use `example_queries` for few-shot prompting.

- **`valyu-pp-cli datasources list`** - Discover all data sources available through Valyu's search API. Each datasource includes metadata, example queries (optimized for few-shot prompting), pricing information, and response schemas.

Use this endpoint to build dynamic tool manifests for AI agents instead of hardcoding knowledge of available datasets.
- **`valyu-pp-cli datasources list-categories`** - Retrieve all available datasource categories. Use these to filter the datasources endpoint.

Categories include: `research`, `healthcare`, `markets`, `company`, `economic`, `predictions`, `transportation`, `legal`, `politics`, `patents`.

### deepresearch

Async research agents that perform comprehensive, multi-step research. DeepResearch searches multiple sources, analyzes content, and generates detailed reports with citations. Tasks run in the background and can take minutes to complete.

- **`valyu-pp-cli deepresearch add-batch-tasks`** - Add up to 100 research tasks to an existing batch. The batch must be in `open` or `processing` status.
- **`valyu-pp-cli deepresearch cancel-batch`** - Cancel all queued and running tasks in a batch. The batch must be in `open` or `processing` status.
- **`valyu-pp-cli deepresearch cancel-deep-research`** - Cancel a DeepResearch task that is currently running or queued. Completed, failed, or already cancelled tasks cannot be cancelled.
- **`valyu-pp-cli deepresearch create-batch`** - Create a batch for running multiple DeepResearch tasks with shared configuration. After creation, add tasks with the Add Tasks endpoint. The batch transitions from `open` to `processing` when tasks are added.
- **`valyu-pp-cli deepresearch create-deep-research`** - Start an async research task that performs comprehensive, multi-step research. The research agent searches multiple sources, analyzes content, executes code, and generates a detailed report with citations.

Tasks run in the background and typically take 1-10 minutes depending on the `mode`. Poll the status endpoint or use webhooks to get results.
- **`valyu-pp-cli deepresearch delete-deep-research`** - Permanently delete a DeepResearch task and its associated data (output, sources, PDF, images). Running or queued tasks must be cancelled first.
- **`valyu-pp-cli deepresearch get-batch-status`** - Retrieve the current status, task counts, and aggregated cost of a batch.
- **`valyu-pp-cli deepresearch get-deep-research-status`** - Retrieve the current status, progress, and results of a DeepResearch task. Poll this endpoint to track research progress and retrieve the final output.
- **`valyu-pp-cli deepresearch list-batch-tasks`** - Retrieve tasks within a batch with optional status filtering and cursor-based pagination. Use `include_output=true` to get full output, sources, images, and cost for each task. When `include_output` is false (default), returns a lightweight listing with status only.
- **`valyu-pp-cli deepresearch list-batches`** - Retrieve all batches for the authenticated API key.
- **`valyu-pp-cli deepresearch list-deep-research-tasks`** - Retrieve a list of all DeepResearch tasks for the authenticated API key.
- **`valyu-pp-cli deepresearch respond-to-deep-research`** - Respond to a human-in-the-loop checkpoint. The task must be in `awaiting_input` or `paused` status. Pass the `interaction_id` from the task's `interaction` field along with the response data matching the checkpoint type.
- **`valyu-pp-cli deepresearch toggle-deep-research-public`** - Toggle the public visibility of a DeepResearch task. When set to `true`, the task results become accessible via a public share link without authentication. Only the task owner can modify the public flag.
- **`valyu-pp-cli deepresearch update-deep-research`** - Send a follow-up instruction to a running DeepResearch task. The instruction can only be added before the writing phase begins. Use this to steer the research in a specific direction based on intermediate findings.

### semantic_search

Manage semantic search

- **`valyu-pp-cli semantic-search`** - Search the web, academic journals, financial databases, and 40+ proprietary data sources. Returns ranked results with extracted content, ready for RAG pipelines and AI agents.

Results are ranked by relevance using semantic reranking (unless `fast_mode` is enabled). You only pay for results returned - pricing is CPM-based (cost per mille tokens).

**Integrating search into an agent?** Consider [DeepResearch](https://docs.valyu.ai/guides/deepresearch) - a cost-effective autonomous agent built on top of this search engine, purpose-built for knowledge work.

### workflows

**Beta.** Templated, versioned DeepResearch for repeatable knowledge work - earnings previews, diligence reads, market sizings. A workflow bundles a prompt template with typed variables, a research strategy, report format, deliverables, tools, and a recommended mode. Workflows are immutably versioned; runs can pin a version. Two scopes exist: your organization's private workflows and curated Valyu workflows available to everyone. Run a workflow by passing `workflow_id` + `workflow_params` to `POST /v1/deepresearch/tasks` - same auth, billing, and task lifecycle as freeform DeepResearch.

- **`valyu-pp-cli workflows create`** - **Beta.** Create a private workflow owned by your organization. Subject to your org's workflow quota (default 100). Returns `409 workflow_quota_exceeded` when the quota is reached.
- **`valyu-pp-cli workflows delete`** - **Beta.** Soft-delete an organization workflow. Valyu-curated workflows cannot be deleted and return `403 cannot_delete_valyu_workflow`.
- **`valyu-pp-cli workflows get`** - **Beta.** Retrieve a single workflow with its resolved version. By default returns the current version; pass `version` to fetch a specific one.
- **`valyu-pp-cli workflows list`** - **Beta.** List workflows visible to your organization. Includes both your org's private workflows and curated Valyu workflows. Run a workflow by passing its `slug` as `workflow_id` to `POST /v1/deepresearch/tasks`.
- **`valyu-pp-cli workflows update`** - **Beta.** Edit an organization workflow, publishing a new immutable version. A `changelog` is required (`changelog_required` otherwise). Valyu-curated workflows are read-only and return `403 cannot_edit_valyu_workflow`.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
valyu-pp-cli datasources list

# JSON for scripting and agents
valyu-pp-cli datasources list --json

# Filter to specific fields
valyu-pp-cli datasources list --json --select id,name,status

# Dry run — show the request without sending
valyu-pp-cli datasources list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
valyu-pp-cli datasources list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
valyu-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/valyu-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `VALYU_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `valyu-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `valyu-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $VALYU_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

### API-specific
- **Error: VALYU_API_KEY not set** — export VALYU_API_KEY=<your-key> or run: valyu-pp-cli auth set-token <key>
- **semantic-search returns 0 results** — Try --search-type all instead of a specific type, or broaden the query
- **deepresearch queue hangs** — Check task status with: valyu-pp-cli deepresearch queue status --agent
