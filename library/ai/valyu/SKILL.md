---
name: pp-valyu
description: "Printing Press CLI for Valyu. The search API built for AI agents."
author: "Anchal Sharma"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - valyu-pp-cli
    install:
      - kind: go
        bins: [valyu-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/valyu/cmd/valyu-pp-cli
---

# Valyu — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `valyu-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install valyu --cli-only
   ```
2. Verify: `valyu-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/valyu/cmd/valyu-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

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

All endpoints require an API key passed via the `X-API-Key` header (the lowercase `x-api-key` form is also accepted). Get your key at [platform.valyu.ai](https://platform.valyu.ai).

```
X-API-Key: your_api_key_here
```

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

## Unique Capabilities

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

## Recipes

### Literature review on a topic

```bash
valyu-pp-cli semantic-search --query "mRNA vaccine efficacy COVID-19" --search-type paper --max-num-results 20 --agent --select url,title,published_date
```

Pull the top 20 academic papers on a topic, selecting only URL, title, and date to minimize token usage.

### SEC filing research

```bash
valyu-pp-cli semantic-search --query "Nvidia 10-K 2024 risk factors" --search-type sec --agent --deliver file:/tmp/sec-results.json
```

Search SEC filings and write results to a file for further analysis.

### Compare web vs academic coverage

```bash
valyu-pp-cli semantic-search compare --query "AI regulation developments 2025" --config-a "type=paper,max=10" --config-b "type=web,max=10" --agent
```

Side-by-side comparison of academic and web sources for a policy topic.

### Cited answer with BibTeX export

```bash
valyu-pp-cli cited-answer --query "What is the current scientific consensus on long COVID?" --cite-output refs.bib --format bibtex --agent
```

Get a researched answer with citations saved as BibTeX for academic reference management.

### Track search spend

```bash
valyu-pp-cli cost --agent --breakdown
```

Monitor cumulative API spend with per-command subtotals.

## Command Reference

**answer** — Get AI-powered answers grounded in real-time search results. The Answer API searches across web, academic, and financial sources, then uses AI to generate a readable response via Server-Sent Events streaming.

- `valyu-pp-cli answer` — Searches across web, academic, and financial sources, then uses AI to generate a readable, cited response.

**contents** — Extract clean, structured content from web pages at scale. Supports batch processing, AI-powered summaries, structured extraction via JSON schemas, and async jobs for large URL sets.

- `valyu-pp-cli contents extract` — Turn any web page into clean, structured data.
- `valyu-pp-cli contents get-job` — Check the status and retrieve results of an async content extraction job.

**datasources** — Discover available data sources and their schemas. A tool manifest for AI agents - instead of hardcoding knowledge of available datasets, agents can query this API to discover sources, filter by category, and use `example_queries` for few-shot prompting.

- `valyu-pp-cli datasources list` — Discover all data sources available through Valyu's search API.
- `valyu-pp-cli datasources list-categories` — Retrieve all available datasource categories. Use these to filter the datasources endpoint.

**deepresearch** — Async research agents that perform comprehensive, multi-step research. DeepResearch searches multiple sources, analyzes content, and generates detailed reports with citations. Tasks run in the background and can take minutes to complete.

- `valyu-pp-cli deepresearch add-batch-tasks` — Add up to 100 research tasks to an existing batch. The batch must be in `open` or `processing` status.
- `valyu-pp-cli deepresearch cancel-batch` — Cancel all queued and running tasks in a batch. The batch must be in `open` or `processing` status.
- `valyu-pp-cli deepresearch cancel-deep-research` — Cancel a DeepResearch task that is currently running or queued.
- `valyu-pp-cli deepresearch create-batch` — Create a batch for running multiple DeepResearch tasks with shared configuration.
- `valyu-pp-cli deepresearch create-deep-research` — Start an async research task that performs comprehensive, multi-step research.
- `valyu-pp-cli deepresearch delete-deep-research` — Permanently delete a DeepResearch task and its associated data (output, sources, PDF, images).
- `valyu-pp-cli deepresearch get-batch-status` — Retrieve the current status, task counts, and aggregated cost of a batch.
- `valyu-pp-cli deepresearch get-deep-research-status` — Retrieve the current status, progress, and results of a DeepResearch task.
- `valyu-pp-cli deepresearch list-batch-tasks` — Retrieve tasks within a batch with optional status filtering and cursor-based pagination.
- `valyu-pp-cli deepresearch list-batches` — Retrieve all batches for the authenticated API key.
- `valyu-pp-cli deepresearch list-deep-research-tasks` — Retrieve a list of all DeepResearch tasks for the authenticated API key.
- `valyu-pp-cli deepresearch respond-to-deep-research` — Respond to a human-in-the-loop checkpoint. The task must be in `awaiting_input` or `paused` status.
- `valyu-pp-cli deepresearch toggle-deep-research-public` — Toggle the public visibility of a DeepResearch task.
- `valyu-pp-cli deepresearch update-deep-research` — Send a follow-up instruction to a running DeepResearch task.

**semantic_search** — Manage semantic search

- `valyu-pp-cli semantic-search` — Search the web, academic journals, financial databases, and 40+ proprietary data sources.

**workflows** — **Beta.** Templated, versioned DeepResearch for repeatable knowledge work - earnings previews, diligence reads, market sizings. A workflow bundles a prompt template with typed variables, a research strategy, report format, deliverables, tools, and a recommended mode. Workflows are immutably versioned; runs can pin a version. Two scopes exist: your organization's private workflows and curated Valyu workflows available to everyone. Run a workflow by passing `workflow_id` + `workflow_params` to `POST /v1/deepresearch/tasks` - same auth, billing, and task lifecycle as freeform DeepResearch.

- `valyu-pp-cli workflows create` — **Beta.** Create a private workflow owned by your organization. Subject to your org's workflow quota (default 100).
- `valyu-pp-cli workflows delete` — **Beta.** Soft-delete an organization workflow.
- `valyu-pp-cli workflows get` — **Beta.** Retrieve a single workflow with its resolved version.
- `valyu-pp-cli workflows list` — **Beta.** List workflows visible to your organization.
- `valyu-pp-cli workflows update` — **Beta.** Edit an organization workflow, publishing a new immutable version.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
valyu-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Set VALYU_API_KEY from app.valyu.world. The CLI reads it automatically; run 'valyu-pp-cli auth set-token <key>' to persist it.

Run `valyu-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  valyu-pp-cli datasources list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
valyu-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
valyu-pp-cli feedback --stdin < notes.txt
valyu-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/valyu-pp-cli/feedback.jsonl`. They are never POSTed unless `VALYU_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VALYU_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
valyu-pp-cli profile save briefing --json
valyu-pp-cli --profile briefing datasources list
valyu-pp-cli profile list --json
valyu-pp-cli profile show briefing
valyu-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `valyu-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/valyu/cmd/valyu-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add valyu-pp-mcp -- valyu-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which valyu-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   valyu-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `valyu-pp-cli <command> --help`.
