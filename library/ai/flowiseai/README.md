# Flowise CLI

**Drives a Flowise REST instance from the shell: run chatflows, compose multi-section newsletters, ingest folders into RAG document stores, replay and audit prior predictions.**

FlowiseAI gives you a visual builder and a REST runtime for chatflows, assistants, and RAG. This CLI is the single binary that lets an agent (or a human) drive that runtime end-to-end: compose a multi-section newsletter, ingest a folder of source material, replay any prediction by chatId, and audit every response against the source documents it cited.

## Install

The recommended path installs both the `flowiseai-pp-cli` binary and the `pp-flowiseai` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install flowiseai
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install flowiseai --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flowiseai-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-flowiseai --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-flowiseai --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-flowiseai skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-flowiseai. The skill defines how its required CLI can be installed.
```

## Authentication

Set FLOWISE_API_KEY to your instance's bearer token and FLOWISE_BASE_URL to your Flowise host (e.g. http://localhost:3000 or https://cloud.flowiseai.com). Use `flowiseai-pp-cli auth set-token <key>` to persist the token in the local config, and `flowiseai-pp-cli auth status` to confirm it's wired.

## Quick Start

```bash
# Confirm host + auth are wired.
flowiseai-pp-cli ping --json


# Verify auth, host, and local store path are all wired correctly.
flowiseai-pp-cli doctor --json


# See what chatflows are deployed on this instance.
flowiseai-pp-cli chatflows list --json --select id,name,deployed


# Send a question to a chatflow and capture the response + chatId.
flowiseai-pp-cli predict abc123 --question "Summarize this week's listings" --json --select text,chatId


# Fan out across the plan's section chatflows and assemble the draft.
flowiseai-pp-cli newsletter compose --plan newsletter.yml --out draft.md


# Search every recorded prediction full-text for a topic.
flowiseai-pp-cli predict search 'mortgage rate' --since 7d --json --select chatId,text

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Compound workflows
- **`newsletter compose`** — Author a multi-section newsletter by fanning out across N chatflows in one command — perfect for an agent assembling a weekly market report.

  _Reach for this when an agent needs to assemble a structured multi-section document from several specialized chatflows in one shot._

  ```bash
  flowiseai-pp-cli newsletter compose --plan newsletter.yml --out draft.md --json
  ```
- **`predict batch`** — Run a chatflow against a CSV or NDJSON of questions concurrently and stream back NDJSON results.

  _Reach for this when fanning a single chatflow across many inputs (per-listing summaries, per-neighborhood briefs, bulk Q&A)._

  ```bash
  flowiseai-pp-cli predict batch abc123 --input listings.csv --concurrency 4 --out results.ndjson --agent
  ```
- **`docstore ingest`** — Walk a folder of source material and ingest every matching file into a document store in one call, then trigger vector indexing.

  _Reach for this when fresh source material arrives in a folder and an agent needs it indexed without writing glue code._

  ```bash
  flowiseai-pp-cli docstore ingest store-xyz ./mls-exports --pattern '*.pdf' --vector-upsert --json
  ```

### Local state that compounds
- **`predict search`** — Full-text search across every recorded prediction with filters on time window, cited document store, used tool, and chatflow.

  _Reach for this when auditing what an agent generated, debugging a hallucination, or filtering predictions by which tools or sources they invoked._

  ```bash
  flowiseai-pp-cli predict search 'mortgage rate' --since 7d --used-tool SendGrid --json --select chatId,text
  ```
- **`newsletter audit`** — Per-chatId audit report joining predictions, chat messages, and upsert history; CSV-friendly for compliance archives.

  _Reach for this at compliance review time or after a newsletter run to verify every section's provenance._

  ```bash
  flowiseai-pp-cli newsletter audit --since 7d --format csv > audit.csv
  ```
- **`docstore drift`** — Show which document stores got new content this week and which chatflows reference each store.

  _Reach for this when investigating why a chatflow's responses changed — drift in the corpus is often the cause._

  ```bash
  flowiseai-pp-cli docstore drift --since 7d --json
  ```
- **`predict replay`** — Re-fire a prior prediction by chatId; optionally diff the new response against the recorded one.

  _Reach for this to recover from a failed agent run or to compare current chatflow output against a known-good baseline._

  ```bash
  flowiseai-pp-cli predict replay cm12345 --diff --json
  ```
- **`predict resume`** — Resume a suspended AgentFlow V2 run by chatId with a structured `humanInput` payload.

  _Reach for this when a chatflow paused awaiting human approval and an agent needs to drive the resume._

  ```bash
  flowiseai-pp-cli predict resume cm67890 --input '{"type":"proceed","feedback":"Approved by Sam"}'
  ```
- **`chatflow deps`** — Show every tool, assistant, variable, and document store referenced by a chatflow; flag missing references.

  _Reach for this before deleting or reviewing a chatflow to know what depends on it._

  ```bash
  flowiseai-pp-cli chatflow deps abc123 --json --show-overrides
  ```
- **`chatflow stale`** — List chatflows not updated in N days, sortable by staleness.

  _Reach for this during housekeeping to find flows that are likely safe to delete._

  ```bash
  flowiseai-pp-cli chatflow stale --days 60 --json
  ```

## Usage

Run `flowiseai-pp-cli --help` for the full command reference and flag list.

## Commands

### assistants

Manage assistants

- **`flowiseai-pp-cli assistants create`** - Create a new assistant with the provided details
- **`flowiseai-pp-cli assistants delete`** - Delete an assistant by ID
- **`flowiseai-pp-cli assistants get-by-id`** - Retrieve a specific assistant by ID
- **`flowiseai-pp-cli assistants list`** - Retrieve a list of all assistants
- **`flowiseai-pp-cli assistants update`** - Update the details of an existing assistant

### attachments

Manage attachments

- **`flowiseai-pp-cli attachments create`** - Return contents of the files in plain string format

### chatflows

Manage chatflows

- **`flowiseai-pp-cli chatflows create`** - Create a new chatflow with the provided details
- **`flowiseai-pp-cli chatflows delete`** - Delete a chatflow by ID
- **`flowiseai-pp-cli chatflows get-by-api-key`** - Retrieve a chatflow using an API key
- **`flowiseai-pp-cli chatflows get-by-id`** - Retrieve a specific chatflow by ID
- **`flowiseai-pp-cli chatflows list`** - Retrieve a list of all chatflows
- **`flowiseai-pp-cli chatflows update`** - Update the details of an existing chatflow

### chatmessage

Manage chatmessage

- **`flowiseai-pp-cli chatmessage get-all-chat-messages`** - Retrieve all chat messages for a specific chatflow.
- **`flowiseai-pp-cli chatmessage remove-all-chat-messages`** - Delete all chat messages for a specific chatflow.

### document-store

Manage document store

- **`flowiseai-pp-cli document-store create`** - Creates a new document store with the provided details
- **`flowiseai-pp-cli document-store delete`** - Deletes a document store by its ID
- **`flowiseai-pp-cli document-store delete-file-chunk`** - Delete a specific chunk from a document loader
- **`flowiseai-pp-cli document-store delete-loader-from`** - Delete specific document loader and associated chunks from document store. This does not delete data from vector store.
- **`flowiseai-pp-cli document-store delete-vector-store-from-store`** - Only data that were upserted with Record Manager will be deleted from vector store
- **`flowiseai-pp-cli document-store edit-file-chunk`** - Updates a specific chunk from a document loader
- **`flowiseai-pp-cli document-store get-all`** - Retrieves a list of all document stores
- **`flowiseai-pp-cli document-store get-by-id`** - Retrieves details of a specific document store by its ID
- **`flowiseai-pp-cli document-store get-file-chunks`** - Get chunks from a specific document loader within a document store
- **`flowiseai-pp-cli document-store query-vector-store`** - Retrieval query for the upserted chunks
- **`flowiseai-pp-cli document-store refresh-document`** - Re-process and upsert all existing documents in document store
- **`flowiseai-pp-cli document-store update`** - Updates the details of a specific document store by its ID
- **`flowiseai-pp-cli document-store upsert-document`** - Upsert document to document store

### flowiseai-feedback

Manage flowiseai feedback

- **`flowiseai-pp-cli flowiseai-feedback create-chat-message-for-chatflow`** - Create new feedback for a specific chat flow.
- **`flowiseai-pp-cli flowiseai-feedback get-all-chat-message`** - List all chat message feedbacks for a chatflow
- **`flowiseai-pp-cli flowiseai-feedback update-chat-message-for-chatflow`** - Update chat message feedback

### leads

Manage leads

- **`flowiseai-pp-cli leads create`** - Create a new lead associated with a specific chatflow
- **`flowiseai-pp-cli leads get-all-for-chatflow`** - Retrieve all leads associated with a specific chatflow

### ping

Manage ping

- **`flowiseai-pp-cli ping server`** - Ping the server to check if it is running

### prediction

Manage prediction

- **`flowiseai-pp-cli prediction create`** - Send a message to your flow and receive an AI-generated response. This is the primary endpoint for interacting with your flows and assistants.
**Authentication**: API key may be required depending on flow settings.

### tools

Manage tools

- **`flowiseai-pp-cli tools create`** - Create a new tool
- **`flowiseai-pp-cli tools delete`** - Delete a specific tool by ID
- **`flowiseai-pp-cli tools get-all`** - Retrieve a list of all tools
- **`flowiseai-pp-cli tools get-by-id`** - Retrieve a specific tool by ID
- **`flowiseai-pp-cli tools update`** - Update a specific tool by ID

### upsert-history

Manage upsert history

- **`flowiseai-pp-cli upsert-history get-all`** - Retrieve all upsert history records with optional filters
- **`flowiseai-pp-cli upsert-history patch-delete`** - Soft delete upsert history records by IDs

### variables

Manage variables

- **`flowiseai-pp-cli variables create`** - Create a new variable
- **`flowiseai-pp-cli variables delete`** - Delete a specific variable by ID
- **`flowiseai-pp-cli variables get-all`** - Retrieve a list of all variables
- **`flowiseai-pp-cli variables update`** - Update a specific variable by ID

### vector

Manage vector

- **`flowiseai-pp-cli vector upsert`** - Upsert vector embeddings of documents in a chatflow


## Cookbook

Real-world recipes for the workflows this CLI was built for.

### Compose a realtor newsletter from a plan

```bash
flowiseai-pp-cli newsletter compose --plan plans/weekly.yml --out drafts/$(date +%F).md --json --select sections.chatId,sections.text
```

Plans declare section name + chatflowId + question + overrideConfig. The CLI fans out and writes the assembled draft plus a JSON manifest of every chatId for downstream audit.

### Ingest a folder of MLS exports into a document store

```bash
flowiseai-pp-cli docstore ingest store-realtor-data ./mls-exports --pattern '*.pdf' --vector-upsert --json
```

Walks the folder, uploads each PDF via multipart, then triggers vector indexing. Records one upsert-history row per file for the drift report.

### Audit last week's newsletter run

```bash
flowiseai-pp-cli newsletter audit --since 7d --format csv > audit.csv
```

CSV-formatted join of predictions, chat messages, and upsert history. One row per chatId with cited document count and tools invoked — drop-in for compliance archives.

### Search recorded predictions for tool usage

```bash
flowiseai-pp-cli predict search 'sendgrid' --since 30d --json --select chatId,text,usedTools
```

FTS5 over local predictions store with JSON1 filter on usedTools — ideal for finding every time an agent dispatched an email.

### Resume a paused AgentFlow V2 run

```bash
flowiseai-pp-cli predict resume cm67890 --input '{"type":"proceed","feedback":"Approved by Sam"}'
```

Looks up the chatflowId from the chatId in the local cache and re-fires the prediction with humanInput populated to continue past the HITL checkpoint.

### Replay a prior prediction and diff against the original

```bash
flowiseai-pp-cli predict replay cm12345 --diff --json
```

Re-fires a prediction by chatId. Pair with `--diff` to compare current chatflow output against the recorded baseline — fastest way to detect chatflow regressions after a tool or prompt change.

### Bulk-predict a CSV of questions concurrently

```bash
flowiseai-pp-cli predict batch abc123 --input listings.csv --concurrency 4 --out results.ndjson --agent
```

Streams NDJSON results as each row completes. Use `--concurrency` to tune for rate limits; output is line-delimited so it pipes directly to `jq`.

### Find stale chatflows during housekeeping

```bash
flowiseai-pp-cli chatflow stale --days 60 --json
```

Lists chatflows untouched in 60+ days, sortable by staleness. Pair with `chatflow deps` before deleting to confirm nothing depends on the flow.

### Inspect what a chatflow depends on

```bash
flowiseai-pp-cli chatflow deps abc123 --json --show-overrides
```

Surfaces every tool, assistant, variable, and document store the chatflow references — flags missing references so you know what to fix before deploying.

### See which document stores changed this week

```bash
flowiseai-pp-cli docstore drift --since 7d --json
```

Surfaces stores that got new content recently and the chatflows that reference each store. First place to look when a chatflow's responses start drifting.

### Sync once, query offline

```bash
flowiseai-pp-cli sync --full --json
flowiseai-pp-cli search 'mortgage rate' --json
```

Pulls every chatflow, assistant, tool, variable, document store, and prediction into a local SQLite. After sync, read commands fall back to local automatically when the API is unreachable; force one or the other with `--data-source local|live`.

### Ping then doctor — the agent's preflight

```bash
flowiseai-pp-cli ping --json && flowiseai-pp-cli doctor --json
```

`ping` confirms the host is reachable; `doctor` confirms auth, base URL, and the local store path are wired. Run both at the top of any agent task so failures surface before the first real call.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
flowiseai-pp-cli assistants list

# JSON for scripting and agents
flowiseai-pp-cli assistants list --json

# Filter to specific fields
flowiseai-pp-cli assistants list --json --select id,name,status

# Dry run — show the request without sending
flowiseai-pp-cli assistants list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
flowiseai-pp-cli assistants list --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-flowiseai -g
```

Then invoke `/pp-flowiseai <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add flowiseai flowiseai-pp-mcp -e FLOWISE_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flowiseai-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FLOWISE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flowiseai": {
      "command": "flowiseai-pp-mcp",
      "env": {
        "FLOWISE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
flowiseai-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/flowiseai-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FLOWISE_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `flowiseai-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FLOWISE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any command** — Confirm FLOWISE_API_KEY is set; some chatflows require a per-flow key — fetch it via `chatflows get-by-api-key <key>` and re-export FLOWISE_API_KEY to that value, or persist with `auth set-token <key>`.
- **Connection refused** — Set FLOWISE_BASE_URL to your Flowise host (default http://localhost:3000). Confirm the host is reachable with `flowiseai-pp-cli ping`.
- **predict returns empty text with sourceDocuments populated** — The chatflow ran retrieval but didn't get to LLM generation. Check overrideConfig for prompt template misconfiguration; pass --explain-config via `chatflow deps --show-overrides`.
- **Newsletter compose section returns 404 on chatflowId** — Run `flowiseai-pp-cli sync` to refresh local cache, then `chatflows list --json --select id,name` and grep client-side to confirm the flow name maps to the right id.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**MilesP46/FlowiseAI-MCP**](https://github.com/MilesP46/FlowiseAI-MCP) — Python
- [**matthewhand/mcp-flowise**](https://github.com/matthewhand/mcp-flowise) — Python
- [**wksbx/flowise-mcp-server**](https://github.com/wksbx/flowise-mcp-server) — TypeScript
- [**FlowiseAI/FlowiseSDK**](https://github.com/FlowiseAI/FlowiseSDK) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
