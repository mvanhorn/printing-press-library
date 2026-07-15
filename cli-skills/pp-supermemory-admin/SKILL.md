---
name: pp-supermemory-admin
description: "Printing Press CLI for Supermemory Admin. The Memory API for the AI era"
author: "Hiten Shah"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - supermemory-admin-pp-cli
    install:
      - kind: go
        bins: [supermemory-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/supermemory-admin/cmd/supermemory-admin-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/supermemory-admin/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Supermemory Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `supermemory-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install supermemory-admin --cli-only
   ```
2. Verify: `supermemory-admin-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/supermemory-admin/cmd/supermemory-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Supermemory's API is powerful, but agents need a repeatable operator surface: auth setup, project scoping, dry runs, compact JSON output, local sync/search, and MCP parity.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Operator Safety
- **`SUPERMEMORY_ADMIN_PROJECT=<project-id> supermemory-admin-pp-cli supermemory-recall`** — Scope every CLI and MCP request to one Supermemory project with the x-sm-project header, without hand-editing raw headers.

  _Keeps agent memories partitioned by project/codebase while preserving the same CLI surface._

  ```bash
  SUPERMEMORY_ADMIN_PROJECT=project_123 supermemory-admin-pp-cli supermemory-recall --q "deployment context" --agent
  ```

### Recall
- **`supermemory-recall`** — Run low-latency memory recall from a compact, agent-friendly CLI surface with selectable JSON output.

  _Lets agents retrieve focused memory context without opening a dashboard or carrying broad history in prompt context._

  ```bash
  supermemory-admin-pp-cli supermemory-recall --q "deployment context" --agent --select results.id,results.memory,results.similarity
  ```

### Local Intelligence
- **`sync + search`** — Sync compatible Supermemory resources into local SQLite for offline search and inspection.

  _Gives operators a local audit/search loop for memory-adjacent resources._

  ```bash
  supermemory-admin-pp-cli sync --agent && supermemory-admin-pp-cli search "project context" --agent
  ```

## Recipes

### Search project-scoped memory

```bash
supermemory-admin-pp-cli supermemory-recall --q "launch notes" --agent --select results.id,results.memory,results.similarity
```

### Audit local synced data

```bash
supermemory-admin-pp-cli sync --agent && supermemory-admin-pp-cli search "customer research" --agent
```

## Command Reference

**connection_resources** — Manage connection resources

- `supermemory-admin-pp-cli connection-resources <connectionId>` — Fetch resources for a connection (supported providers: GitHub for now)

**connections** — External service integrations

- `supermemory-admin-pp-cli connections delete-v3-by-id` — Delete a specific connection by ID
- `supermemory-admin-pp-cli connections delete-v3-by-provider` — Delete connection for a specific provider and container tags
- `supermemory-admin-pp-cli connections get-v3-by-id` — Get connection details with id
- `supermemory-admin-pp-cli connections post-v3-by-provider` — Initialize connection and get authorization URL
- `supermemory-admin-pp-cli connections post-v3-list` — List all connections

**container-tags** — Manage container tags

- `supermemory-admin-pp-cli container-tags delete-v3-by` — Delete a container tag and all its documents and memories. Only organization owners and admins can perform this action.
- `supermemory-admin-pp-cli container-tags get-v3-by` — Get settings for a container tag
- `supermemory-admin-pp-cli container-tags patch-v3-by` — Update settings for a container tag
- `supermemory-admin-pp-cli container-tags post-v3-merge` — Merge multiple container tags into a target tag.

**conversations** — Manage conversations

- `supermemory-admin-pp-cli conversations` — Ingest or update a conversation

**documents** — List, get, and search documents

- `supermemory-admin-pp-cli documents delete-v3-bulk` — Bulk delete documents by IDs or container tags
- `supermemory-admin-pp-cli documents delete-v3-by-id` — Delete a document by ID or customId
- `supermemory-admin-pp-cli documents get-v3-by-id` — Get a document by ID
- `supermemory-admin-pp-cli documents get-v3-processing` — Get documents that are currently being processed
- `supermemory-admin-pp-cli documents patch-v3-by-id` — Update a document with any content type (text, url, file, etc.) and metadata
- `supermemory-admin-pp-cli documents post-v3` — Add a document with any content type (text, url, file, etc.) and metadata
- `supermemory-admin-pp-cli documents post-v3-batch` — Add multiple documents in a single request. Each document can have any content type (text, url, file, etc.) and metadata
- `supermemory-admin-pp-cli documents post-v3-file` — Upload a file to be processed
- `supermemory-admin-pp-cli documents post-v3-list` — Retrieves a paginated list of documents with their metadata and workflow status
- `supermemory-admin-pp-cli documents post-v3-search` — Search memories with advanced filtering

**memories** — Manage memories

- `supermemory-admin-pp-cli memories delete-v4` — Forget (soft delete) a memory entry. The memory is marked as forgotten but not permanently deleted.
- `supermemory-admin-pp-cli memories patch-v4` — Update a memory by creating a new version. The original memory is preserved with isLatest=false.
- `supermemory-admin-pp-cli memories post-v4` — Create memories directly, bypassing the document ingestion workflow.
- `supermemory-admin-pp-cli memories post-v4-list` — List all latest memory entries from specified container tags with their update history and source documents

**profiles** — Entity profiles for users, participants, or any entity — includes profile search

- `supermemory-admin-pp-cli profiles` — Get user profile with optional search results

**settings** — Organization settings

- `supermemory-admin-pp-cli settings get-v3` — Get settings for an organization
- `supermemory-admin-pp-cli settings patch-v3` — Update settings for an organization
- `supermemory-admin-pp-cli settings post-v3-reset` — Reset organization content: removes documents, memories, spaces (except default project), connections, and org settings.

**supermemory_recall** — Manage supermemory recall

- `supermemory-admin-pp-cli supermemory-recall` — Search memory entries - Low latency for conversational


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
supermemory-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Create a Supermemory API key and set SUPERMEMORY_ADMIN_TOKEN. Optionally set SUPERMEMORY_ADMIN_PROJECT to scope every request with x-sm-project.

Run `supermemory-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  supermemory-admin-pp-cli connection-resources mock-value --agent --select id,name,status
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
supermemory-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
supermemory-admin-pp-cli feedback --stdin < notes.txt
supermemory-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/supermemory-admin-pp-cli/feedback.jsonl`. They are never POSTed unless `SUPERMEMORY_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SUPERMEMORY_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
supermemory-admin-pp-cli profile save briefing --json
supermemory-admin-pp-cli --profile briefing connection-resources mock-value
supermemory-admin-pp-cli profile list --json
supermemory-admin-pp-cli profile show briefing
supermemory-admin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `supermemory-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/supermemory-admin/cmd/supermemory-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add supermemory-admin-pp-mcp -- supermemory-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which supermemory-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   supermemory-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `supermemory-admin-pp-cli <command> --help`.
