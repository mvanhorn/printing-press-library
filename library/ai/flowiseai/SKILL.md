---
name: pp-flowiseai
description: "Drives a Flowise REST instance from the shell: run chatflows, compose multi-section newsletters, ingest folders into RAG document stores, replay and audit prior predictions. Trigger phrases: `send a Flowise prediction`, `run a chatflow`, `compose a newsletter with Flowise`, `ingest documents into a Flowise document store`, `audit Flowise predictions`, `use flowiseai-pp-cli`, `drive my Flowise instance`."
author: "Daniel Larson"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - flowiseai-pp-cli
---

# Flowise — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `flowiseai-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install flowiseai --cli-only
   ```
2. Verify: `flowiseai-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

FlowiseAI gives you a visual builder and a REST runtime for chatflows, assistants, and RAG. This CLI is the single binary that lets an agent (or a human) drive that runtime end-to-end: compose a multi-section newsletter, ingest a folder of source material, replay any prediction by chatId, and audit every response against the source documents it cited.

## When to Use This CLI

Reach for flowiseai-pp-cli when an agent needs to drive a Flowise instance from a shell — composing newsletters, batch-running predictions across a list, ingesting a folder of RAG source material, replaying a prior prediction for audit, or searching across recorded predictions full-text. It is the only Flowise client that ships compound workflows alongside full REST coverage.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

> **Run `flowiseai-pp-cli sync` once before using the local-state commands** (`predict search`, `newsletter audit`, `docstore drift`, `predict replay`, `predict resume`, `chatflow deps`, `chatflow stale`). They read from the locally-synced SQLite cache; without a recent `sync`, results will be empty.

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

## Command Reference

**assistants** — Manage assistants

- `flowiseai-pp-cli assistants create` — Create a new assistant with the provided details
- `flowiseai-pp-cli assistants delete` — Delete an assistant by ID
- `flowiseai-pp-cli assistants get-by-id` — Retrieve a specific assistant by ID
- `flowiseai-pp-cli assistants list` — Retrieve a list of all assistants
- `flowiseai-pp-cli assistants update` — Update the details of an existing assistant

**attachments** — Manage attachments

- `flowiseai-pp-cli attachments <chatflowId> <chatId>` — Return contents of the files in plain string format

**chatflows** — Manage chatflows

- `flowiseai-pp-cli chatflows create` — Create a new chatflow with the provided details
- `flowiseai-pp-cli chatflows delete` — Delete a chatflow by ID
- `flowiseai-pp-cli chatflows get-by-api-key` — Retrieve a chatflow using an API key
- `flowiseai-pp-cli chatflows get-by-id` — Retrieve a specific chatflow by ID
- `flowiseai-pp-cli chatflows list` — Retrieve a list of all chatflows
- `flowiseai-pp-cli chatflows update` — Update the details of an existing chatflow

**chatmessage** — Manage chatmessage

- `flowiseai-pp-cli chatmessage get-all-chat-messages` — Retrieve all chat messages for a specific chatflow.
- `flowiseai-pp-cli chatmessage remove-all-chat-messages` — Delete all chat messages for a specific chatflow.

**document-store** — Manage document store

- `flowiseai-pp-cli document-store create` — Creates a new document store with the provided details
- `flowiseai-pp-cli document-store delete` — Deletes a document store by its ID
- `flowiseai-pp-cli document-store delete-file-chunk` — Delete a specific chunk from a document loader
- `flowiseai-pp-cli document-store delete-loader-from` — Delete specific document loader and associated chunks from document store. This does not delete data from vector store.
- `flowiseai-pp-cli document-store delete-vector-store-from-store` — Only data that were upserted with Record Manager will be deleted from vector store
- `flowiseai-pp-cli document-store edit-file-chunk` — Updates a specific chunk from a document loader
- `flowiseai-pp-cli document-store get-all` — Retrieves a list of all document stores
- `flowiseai-pp-cli document-store get-by-id` — Retrieves details of a specific document store by its ID
- `flowiseai-pp-cli document-store get-file-chunks` — Get chunks from a specific document loader within a document store
- `flowiseai-pp-cli document-store query-vector-store` — Retrieval query for the upserted chunks
- `flowiseai-pp-cli document-store refresh-document` — Re-process and upsert all existing documents in document store
- `flowiseai-pp-cli document-store update` — Updates the details of a specific document store by its ID
- `flowiseai-pp-cli document-store upsert-document` — Upsert document to document store

**flowiseai-feedback** — Manage flowiseai feedback

- `flowiseai-pp-cli flowiseai-feedback create-chat-message-for-chatflow` — Create new feedback for a specific chat flow.
- `flowiseai-pp-cli flowiseai-feedback get-all-chat-message` — List all chat message feedbacks for a chatflow
- `flowiseai-pp-cli flowiseai-feedback update-chat-message-for-chatflow` — Update chat message feedback

**leads** — Manage leads

- `flowiseai-pp-cli leads create` — Create a new lead associated with a specific chatflow
- `flowiseai-pp-cli leads get-all-for-chatflow` — Retrieve all leads associated with a specific chatflow

**ping** — Manage ping

- `flowiseai-pp-cli ping` — Ping the server to check if it is running

**prediction** — Manage prediction

- `flowiseai-pp-cli prediction <id>` — Send a message to your flow and receive an AI-generated response. This is the primary endpoint for interacting with...

**tools** — Manage tools

- `flowiseai-pp-cli tools create` — Create a new tool
- `flowiseai-pp-cli tools delete` — Delete a specific tool by ID
- `flowiseai-pp-cli tools get-all` — Retrieve a list of all tools
- `flowiseai-pp-cli tools get-by-id` — Retrieve a specific tool by ID
- `flowiseai-pp-cli tools update` — Update a specific tool by ID

**upsert-history** — Manage upsert history

- `flowiseai-pp-cli upsert-history get-all` — Retrieve all upsert history records with optional filters
- `flowiseai-pp-cli upsert-history patch-delete` — Soft delete upsert history records by IDs

**variables** — Manage variables

- `flowiseai-pp-cli variables create` — Create a new variable
- `flowiseai-pp-cli variables delete` — Delete a specific variable by ID
- `flowiseai-pp-cli variables get-all` — Retrieve a list of all variables
- `flowiseai-pp-cli variables update` — Update a specific variable by ID

**vector** — Manage vector

- `flowiseai-pp-cli vector <id>` — Upsert vector embeddings of documents in a chatflow


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
flowiseai-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Compose a realtor newsletter from a plan

```bash
flowiseai-pp-cli newsletter compose --plan plans/weekly.yml --out drafts/$(date +%F).md --json --select sections.chatId,sections.text
```

Plans declare section name + chatflowId + question + overrideConfig; the CLI fans out and writes the assembled draft plus a JSON manifest of every chatId for downstream audit.

### Ingest a folder of MLS exports

```bash
flowiseai-pp-cli docstore ingest store-realtor-data ./mls-exports --pattern '*.pdf' --vector-upsert --json
```

Walks the folder, uploads each PDF via multipart, then triggers vector indexing; records one upsert-history row per file for the drift report.

### Audit last week's newsletter run

```bash
flowiseai-pp-cli newsletter audit --since 7d --format csv > audit.csv
```

CSV-formatted join of predictions, chat messages, and upsert history; one row per chatId with the cited document count and tools invoked.

### Search recorded predictions for tool usage

```bash
flowiseai-pp-cli predict search 'sendgrid' --since 30d --json --select chatId,text,usedTools
```

FTS5 over local predictions store with JSON1 filter on usedTools; ideal for finding every time the agent dispatched an email.

### Resume a paused AgentFlow V2 run

```bash
flowiseai-pp-cli predict resume cm67890 --input '{"type":"proceed","feedback":"Approved by Sam"}'
```

Looks up the chatflowId from the chatId in local cache and re-fires the prediction with humanInput populated to continue past the HITL checkpoint.

## Auth Setup

Set FLOWISE_API_KEY to your instance's bearer token and FLOWISE_BASE_URL to your Flowise host (e.g. http://localhost:3000 or https://cloud.flowiseai.com). Use `flowiseai-pp-cli auth set-token <key>` to persist the token in the local config, and `flowiseai-pp-cli auth status` to confirm it's wired.

Run `flowiseai-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  flowiseai-pp-cli assistants list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
flowiseai-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
flowiseai-pp-cli feedback --stdin < notes.txt
flowiseai-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.flowiseai-pp-cli/feedback.jsonl`. They are never POSTed unless `FLOWISEAI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `FLOWISEAI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
flowiseai-pp-cli profile save briefing --json
flowiseai-pp-cli --profile briefing assistants list
flowiseai-pp-cli profile list --json
flowiseai-pp-cli profile show briefing
flowiseai-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `flowiseai-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add flowiseai-pp-mcp -- flowiseai-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which flowiseai-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   flowiseai-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `flowiseai-pp-cli <command> --help`.
