# Exa CLI

**Every Exa search API feature, plus local history, spend tracking, and diffing no other Exa tool has.**

exa-pp-cli wraps the full Exa API — search, contents, answer, find-similar, monitors, agent runs, websets, webhooks, imports — and adds a local SQLite store so every result, run, and cost lands on disk. Re-run past queries, diff monitor runs, track entity first-seens, and watch spend in real time.

## Install

The recommended path installs both the `exa-pp-cli` binary and the `pp-exa` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install exa
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install exa --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install exa --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install exa --agent claude-code
npx -y @mvanhorn/printing-press-library install exa --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/exa/cmd/exa-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/exa-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install exa --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-exa --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-exa --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install exa --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/exa-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EXA_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/exa/cmd/exa-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "exa": {
      "command": "exa-pp-mcp",
      "env": {
        "EXA_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Verify the install and API key wiring without spending credits.
exa-pp-cli doctor --dry-run

# Run a semantic web search with key excerpts for agent context.
exa-pp-cli websearch --query "latest developments in LLMs" --num-results 5 --contents '{"highlights":true}'

# Extract clean markdown text from a known URL.
exa-pp-cli contents --urls "https://example.com" --highlights true

# Get a grounded LLM answer with citations.
exa-pp-cli answer --query "What is the capital of France?"

# Persist monitor run history locally, then query it offline.
exa-pp-cli sync --resources monitors --full

# See per-day, per-resource API spend from stored cost data.
exa-pp-cli spend --days 30

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`spend`** — See cumulative API spend across every Exa call, broken down by day and resource.

  _Agents should reach for this when they need to know how much Exa usage costs before running another batch._

  ```bash
  exa-pp-cli spend --days 30 --resource searches
  ```
- **`monitor diff`** — Compare two synced monitor runs and see exactly which URLs are new, gone, or unchanged.

  _Reach for this to see what a scheduled search found since the last run instead of re-reading entire run outputs._

  ```bash
  exa-pp-cli monitor diff <monitor-id>
  ```
- **`entity report`** — Build a first-seen / last-seen / mention-count timeline for any company or person across your synced searches and webset items.

  _Reach for this to track when an entity first appeared and how often it shows up across all your research surfaces._

  ```bash
  exa-pp-cli entity report "Acme Corp" --type company --since 30d
  ```
- **`webset new`** — List items added to a live webset since your last sync, so you only see what changed.

  _Reach for this for the weekly what's-new sweep over a curated set instead of re-listing every item._

  ```bash
  exa-pp-cli webset new <webset-id> --since 7d
  ```

## Recipes

### Deep research with structured output

```bash
exa-pp-cli websearch --query "compare the latest frontier AI model releases" --type deep --output-schema '{"type":"object","required":["models"],"properties":{"models":{"type":"array","items":{"type":"object"}}}}'
```

Run deep search and get a synthesized, schema-shaped answer with grounding.

### Monitor a competitor's news

```bash
exa-pp-cli monitors create --name "competitor-news" --search-query "new product launches" --search-contents-highlights true
```

Schedule a recurring search, then run `sync` to persist runs locally.

### What changed since last monitor run

```bash
exa-pp-cli monitor diff <monitor-id>
```

Schedule a recurring search, then run `sync` to persist runs locally.

### Track an entity over time

```bash
exa-pp-cli entity report "Acme Corp" --since 30d --agent --select entity,mentionCount
```

Get an agent-shaped first-seen/last-seen timeline for a company across all synced research.

### Narrow a search response with --select

```bash
exa-pp-cli websearch --query "AI regulation policy updates" --category news --num-results 5 --select results.title,results.url
```

Request only the high-gravity fields so agent context is not flooded with full result payloads.

## Usage

Run `exa-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, credentials, auth sidecars, and other auth sidecars |
| `state` | Runtime state such as persisted queries, learn journal, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `EXA_CONFIG_DIR`, `EXA_DATA_DIR`, `EXA_STATE_DIR`, or `EXA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `EXA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export EXA_HOME=/srv/exa
exa-pp-cli doctor
```

Under `EXA_HOME=/srv/exa`, the four dirs resolve to `/srv/exa/config`, `/srv/exa/data`, `/srv/exa/state`, and `/srv/exa/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "exa": {
      "command": "exa-pp-mcp",
      "env": {
        "EXA_HOME": "/srv/exa"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `EXA_DATA_DIR` overrides an explicit `--home` for that kind. Use `EXA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `EXA_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `exa-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### agent

Manage agent

- **`exa-pp-cli agent cancel-run`** - Cancel a queued or running Agent run. If the run has already reached a terminal status, the API returns the existing run.
- **`exa-pp-cli agent create-run`** - Create an asynchronous Agent run. By default, the API returns the run object immediately. Set `Accept: text/event-stream` to stream run lifecycle events until the run reaches a terminal status.
- **`exa-pp-cli agent delete-run`** - Delete a stored Agent run.
- **`exa-pp-cli agent get-run`** - Retrieve a single Agent run by ID.
- **`exa-pp-cli agent list-run-events`** - List stored events for an Agent run. Set `Accept: text/event-stream` to replay stored events as server-sent events. Use `cursor` for JSON pagination or `Last-Event-ID` for SSE replay.
- **`exa-pp-cli agent list-runs`** - List Agent runs for your team, ordered from newest to oldest.

### answer

Manage answer

- **`exa-pp-cli answer`** - Performs a search based on the query and generates either a direct answer or a detailed summary with citations, depending on the query type.

### contents

Manage contents

- **`exa-pp-cli contents`** - Contents

### events

Manage events

- **`exa-pp-cli events get`** - Get a single Event by id.

You can subscribe to Events by creating a Webhook.
- **`exa-pp-cli events list`** - List all events that have occurred in the system.

You can paginate through the results using the `cursor` parameter.

### find-similar

Manage find similar

- **`exa-pp-cli find-similar`** - Find links similar to the provided URL and optionally retrieve their contents. Deprecated: prefer `/search` with a query describing the source.

### imports

Manage imports

- **`exa-pp-cli imports create`** - Creates a new import to upload your data into Websets. Imports can be used to:

- **Enrich**: Enhance your data with additional information using our AI-powered enrichment engine
- **Search**: Query your data using Websets' agentic search with natural language filters
- **Exclude**: Prevent duplicate or already known results from appearing in your searches

Once the import is created, you can upload your data to the returned `uploadUrl` until `uploadValidUntil` (by default 1 hour).
- **`exa-pp-cli imports delete`** - Deletes a import.
- **`exa-pp-cli imports get`** - Gets a specific import.
- **`exa-pp-cli imports list`** - Lists all imports for the Webset.
- **`exa-pp-cli imports update`** - Updates a import configuration.

### monitors

Manage monitors

- **`exa-pp-cli monitors batch`** - Perform a batch action on monitors matching the provided filters.

Supported actions:
- **delete**: Permanently remove matching monitors
- **pause**: Pause matching monitors
- **unpause**: Unpause matching monitors

Use `dry_run: true` (the default) to preview which monitors would be affected before performing the action. Results are paginated via the `limit` parameter; loop until `has_more` is `false` to process all matching monitors.
- **`exa-pp-cli monitors create`** - Creates a new Monitor to run recurring Exa searches on a schedule.

Monitors automatically execute your search query on a recurring schedule and deliver results to your webhook endpoint with automatic deduplication:

- **Date-based filtering** only fetches content since the last run

- **Semantic deduplication** tracks previous outputs to surface only new developments

The response includes a `webhookSecret` that is only returned once at creation time. Store it securely for webhook signature verification.
- **`exa-pp-cli monitors create-endpoint`** - Creates a new `Monitor` to continuously keep your Websets updated with fresh data.

Monitors automatically run on your defined schedule to ensure your Websets stay current without manual intervention:

- **Find new content**: Execute `search` operations to discover fresh items matching your criteria
- **Update existing content**: Run `refresh` operations to update items contents and enrichments
- **Automated scheduling**: Configure `cron` expressions and `timezone` for precise scheduling control
- **`exa-pp-cli monitors delete`** - Deletes a monitor. This cannot be undone.
- **`exa-pp-cli monitors delete-id`** - Deletes a monitor.
- **`exa-pp-cli monitors get`** - Retrieves a single monitor by its ID.
- **`exa-pp-cli monitors get-id`** - Gets a specific monitor.
- **`exa-pp-cli monitors list`** - Lists all monitors for the authenticated team. Supports filtering by status and cursor-based pagination.
- **`exa-pp-cli monitors list-endpoint`** - Lists all monitors for the Webset.
- **`exa-pp-cli monitors update`** - Updates an existing monitor. All fields are optional. For `search`, you can send a partial object containing only the fields you want to change. Set `trigger` to `null` to remove the schedule.
- **`exa-pp-cli monitors update-id`** - Updates a monitor configuration.

### teams

Manage teams

- **`exa-pp-cli teams`** - Returns information about the authenticated team, including current concurrency usage and limits.

### webhooks

Manage webhooks

- **`exa-pp-cli webhooks create`** - Create a Webhook
- **`exa-pp-cli webhooks delete`** - Delete a Webhook
- **`exa-pp-cli webhooks get`** - Get a Webhook
- **`exa-pp-cli webhooks list`** - List webhooks
- **`exa-pp-cli webhooks update`** - Update a Webhook

### websearch

Manage websearch

- **`exa-pp-cli websearch`** - Perform a search with an Exa prompt-engineered query and retrieve a list of relevant results. Optionally get contents.

### websets

Manage websets

- **`exa-pp-cli websets create`** - Creates a new Webset with optional search, import, and enrichment configurations. The Webset will automatically begin processing once created.

You can specify an `externalId` to reference the Webset with your own identifiers for easier integration.
- **`exa-pp-cli websets delete`** - Deletes a Webset.

Once deleted, the Webset and all its Items will no longer be available.
- **`exa-pp-cli websets get`** - Get a Webset
- **`exa-pp-cli websets list`** - Returns a list of Websets.

You can paginate through the results using the `cursor` parameter.

You can filter results using the `search` parameter to find Websets by ID, external ID, or title.
- **`exa-pp-cli websets preview`** - Preview how a search query will be decomposed before creating a webset. This endpoint performs the same query analysis that happens during webset creation, allowing you to see the detected entity type, generated search criteria, and available enrichment columns in advance.

Use this to help users understand how their search will be interpreted before committing to a full webset creation.
- **`exa-pp-cli websets update`** - Update a Webset


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`exa-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`exa-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`exa-pp-cli learnings list`** - Inspect taught rows
- **`exa-pp-cli learnings forget <query>`** - Undo a teach
- **`exa-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`exa-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`exa-pp-cli teach-pattern`** - Install a query/resource template up front
- **`exa-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `EXA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `exa-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
exa-pp-cli contents

# JSON for scripting and agents
exa-pp-cli contents --json

# Filter to specific fields
exa-pp-cli contents --json --select id,name,status

# Dry run — show the request without sending
exa-pp-cli contents --dry-run

# Agent mode — JSON + compact + no prompts in one flag
exa-pp-cli contents --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
exa-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `exa-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/exa-pp-cli/config.toml`; `--home`, `EXA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EXA_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `exa-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `exa-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EXA_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 invalid API key** — Set EXA_API_KEY to a key from https://dashboard.exa.ai/api-keys; pass it via Authorization: Bearer or x-api-key.
- **402 NO_MORE_CREDITS** — Top up credits at dashboard.exa.ai or raise the API key spending budget.
- **429 rate limit exceeded** — Search is limited to 10 QPS and contents to 100 QPS; the CLI retries with backoff, slow down parallel loops.
- **contents returns 200 but some URLs failed** — Check the statuses[] array in the response for per-URL error tags like CRAWL_NOT_FOUND.
- **answer returns 501 UNABLE_TO_GENERATE_RESPONSE** — Rephrase the query or adjust parameters; the model could not answer from available information.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**exa-mcp-server**](https://github.com/exa-labs/exa-mcp-server) — TypeScript (4843 stars)
- [**exa-direct**](https://github.com/BjornMelin/exa-direct) — Python (3 stars)
- [**exa-js**](https://github.com/exa-labs/exa-js) — JavaScript
- [**exa-py**](https://github.com/exa-labs/exa-py) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
