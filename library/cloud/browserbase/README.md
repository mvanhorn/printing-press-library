# Browserbase CLI

**Every Browserbase cloud feature, plus session lifecycle control, local history, and usage analytics no other tool has.**

Manage the full Browserbase surface — sessions, projects, downloads, contexts, agents, functions — from one agent-native CLI. Track orphaned sessions, batch-fetch with rate-limit pacing, diff agent runs, and watch usage trends from a local SQLite store that compounds across syncs.

## Install

The recommended path installs both the `browserbase-pp-cli` binary and the `pp-browserbase` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install browserbase
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install browserbase --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install browserbase --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install browserbase --agent claude-code
npx -y @mvanhorn/printing-press-library install browserbase --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/browserbase/cmd/browserbase-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/browserbase-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install browserbase --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-browserbase --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-browserbase --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install browserbase --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/browserbase-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BROWSERBASE_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/browserbase/cmd/browserbase-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "browserbase": {
      "command": "browserbase-pp-mcp",
      "env": {
        "BROWSERBASE_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set BROWSERBASE_API_KEY (your key looks like `bb_live_...`). Optionally set BROWSERBASE_PROJECT_ID to scope commands to a project.

## Quick Start

```bash
# Verify the CLI works and your API key is set.
browserbase-pp-cli doctor --dry-run

# See your projects and find the one to work in.
browserbase-pp-cli projects list

# Spin up a headless browser session.
browserbase-pp-cli sessions create --project-id 1fbe3566-db19-4010-9410-0ba94f0497ea --json

# Find sessions that were never released and are burning minutes.
browserbase-pp-cli sessions orphans --older-than 15m --json

# Grab a page as clean markdown without writing browser code.
browserbase-pp-cli fetch --url https://example.com --format markdown

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Session lifecycle control
- **`sessions orphans`** — Find running sessions that were never released (keepAlive orphans) and the runtime they're burning, then optionally stop them in batch. Requires `sync` first — an unsynced store returns an empty scan.

  _Reach for this when a crashed automation may have left billable sessions running — it turns a silent cost leak into a one-command cleanup._

  ```bash
  browserbase-pp-cli sessions orphans --older-than 15m --stop --json
  ```
- **`sessions run`** — Create a session, print its connect URL, and guarantee it is released on completion, timeout, or interrupt.

  _Use when you need a browser session for a script or agent without leaking keepAlive sessions on failure._

  ```bash
  browserbase-pp-cli sessions run --project 1fbe3566-db19-4010-9410-0ba94f0497ea --timeout 15m
  ```

### Agent-native fetch pipeline
- **`fetch batch`** — Fetch a list of URLs with rate-limit pacing and a resumable checkpoint, so large scrape jobs survive interruptions.

  _Reach for this when scraping a list of pages: it paces requests, skips already-fetched URLs, and reports per-URL status._

  ```bash
  browserbase-pp-cli fetch batch --file urls.txt --format markdown --resume --json
  ```
- **`agents runs diff`** — Compare two agent runs structurally — message sequences and final structured results — to see what changed between prompt iterations.

  _Reach for this when iterating on an agent's prompt or schema and you need to see exactly what changed between two runs._

  ```bash
  browserbase-pp-cli agents runs diff 52f6b13d-eb27-436d-86ff-356b2fd01697 2d310606-42fa-483c-9a7b-7102a85ddb09 --json
  ```

### Local state that compounds
- **`projects digest`** — See everything that ran in a project this week — sessions, agent runs, and downloads grouped by day — from the local store. Requires `sync` first; an unsynced store returns an empty digest.

  _Use when reviewing what a project actually did over a period, without clicking through the dashboard._

  ```bash
  browserbase-pp-cli projects digest --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 7d --json
  ```
- **`usage trend`** — Track per-project browserMinutes and proxyBytes over sync history and spot quota creep before the bill arrives. Requires `sync` first; an unsynced store returns an empty trend.

  _Use when you own the Browserbase bill and want to see usage direction, not just a snapshot._

  ```bash
  browserbase-pp-cli usage trend --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 30d --json
  ```
- **`web history`** — Review past fetch and search calls with cached results, and re-emit a cached response without re-hitting the API. Requires `sync` first; an unsynced store returns an empty history.

  _Reach for this when you need provenance for what was fetched, or want to re-run a prior result offline._

  ```bash
  browserbase-pp-cli web history --since 7d --type fetch --json
  ```

## Recipes

### Clean up orphaned sessions

```bash
browserbase-pp-cli sessions orphans --older-than 15m --stop --json
```

Find and release sessions that were never explicitly stopped, preventing wasted browser minutes.

### Scrape a list of URLs safely

```bash
browserbase-pp-cli fetch batch --file urls.txt --format markdown --resume --json
```

Fetch many pages with rate-limit pacing and resumable progress.

### Weekly project review

```bash
browserbase-pp-cli projects digest --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 7d --json
```

See every session, agent run, and download in the project this week.

### Check usage trend

```bash
browserbase-pp-cli usage trend --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 30d --json
```

Spot quota creep before the bill arrives.

### Narrow a fetch response for agents

```bash
browserbase-pp-cli fetch --url https://news.ycombinator.com --format markdown --agent --select statusCode,markdown
```

Fetch a page and select only the fields an agent needs, saving context.

## Usage

Run `browserbase-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `BROWSERBASE_CONFIG_DIR`, `BROWSERBASE_DATA_DIR`, `BROWSERBASE_STATE_DIR`, or `BROWSERBASE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `BROWSERBASE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export BROWSERBASE_HOME=/srv/browserbase
browserbase-pp-cli doctor
```

Under `BROWSERBASE_HOME=/srv/browserbase`, the four dirs resolve to `/srv/browserbase/config`, `/srv/browserbase/data`, `/srv/browserbase/state`, and `/srv/browserbase/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "browserbase": {
      "command": "browserbase-pp-mcp",
      "env": {
        "BROWSERBASE_HOME": "/srv/browserbase"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `BROWSERBASE_DATA_DIR` overrides an explicit `--home` for that kind. Use `BROWSERBASE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `BROWSERBASE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `browserbase-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### agents

Manage agents

- **`browserbase-pp-cli agents create`** - Create a reusable agent. An agent defines a `systemPrompt` and `resultSchema` that guide its behavior for every run. Only `name` is required; an agent created with no `systemPrompt` behaves like an unconfigured run.
- **`browserbase-pp-cli agents delete`** - Delete an agent. Runs that already referenced this agent are unaffected.
- **`browserbase-pp-cli agents get`** - Retrieve an agent by ID.
- **`browserbase-pp-cli agents list`** - List agents across your account. Supports filtering by creation time.
- **`browserbase-pp-cli agents runs-create`** - Run a browser agent to complete the `task` by using web search and browser tooling. Optionally pass `agentId` to run a [custom agent](/reference/api/create-an-agent) you've created.
- **`browserbase-pp-cli agents runs-get`** - Retrieve the current status and details of a run, including its result and associated session information. To fetch the run's messages, use [List Run Messages](/reference/api/list-run-messages).
- **`browserbase-pp-cli agents runs-list`** - List runs across your account. Supports filtering by status, by the agent they reference, and by creation time.
- **`browserbase-pp-cli agents runs-messages`** - Returns a paginated list of messages produced by a run, in chronological order, with the oldest messages first.

Messages conform to the [AI SDK UIMessage format](https://ai-sdk.dev/docs/reference/ai-sdk-core/ui-message).
- **`browserbase-pp-cli agents runs-stop`** - Request that an in-progress run stop. The run winds down and transitions to `STOPPED`. Stopping a run that has already finished returns a conflict.
- **`browserbase-pp-cli agents update`** - Update an existing agent. Only the fields provided in the body are modified; omitted fields are left unchanged.

### certificates

Manage certificates

- **`browserbase-pp-cli certificates delete`** - Delete a Certificate
- **`browserbase-pp-cli certificates get`** - Get a Certificate
- **`browserbase-pp-cli certificates list`** - List Certificates
- **`browserbase-pp-cli certificates upload`** - Upload a Certificate

### contexts

Manage contexts

- **`browserbase-pp-cli contexts create`** - Create a Context
- **`browserbase-pp-cli contexts delete`** - Delete a Context
- **`browserbase-pp-cli contexts get`** - Get a Context

### downloads

Manage downloads

- **`browserbase-pp-cli downloads delete`** - Delete a download file from storage and mark as deleted.
- **`browserbase-pp-cli downloads get`** - Get download metadata (Accept: application/json) or file content (Accept: application/octet-stream).
- **`browserbase-pp-cli downloads list`** - List all downloads for a session with optional filtering and pagination.

### extensions

Manage extensions

- **`browserbase-pp-cli extensions delete`** - Delete an Extension
- **`browserbase-pp-cli extensions get`** - Get an Extension
- **`browserbase-pp-cli extensions upload`** - Upload an Extension

### fetch

Manage fetch

- **`browserbase-pp-cli fetch`** - Fetch a page and return its content, headers, and metadata.

### functions

Manage functions

- **`browserbase-pp-cli functions builds-get`** - Get a Function Build
- **`browserbase-pp-cli functions builds-get-logs`** - Get Function Build Logs
- **`browserbase-pp-cli functions builds-list`** - List Function Builds
- **`browserbase-pp-cli functions get`** - Get a Function
- **`browserbase-pp-cli functions invocations-get`** - Get an Invocation
- **`browserbase-pp-cli functions invocations-get-logs`** - Get Invocation Logs
- **`browserbase-pp-cli functions list`** - List Functions
- **`browserbase-pp-cli functions versions-get`** - Get a Function Version
- **`browserbase-pp-cli functions versions-list-invocations`** - List Invocations for a Function Version

### projects

Manage projects

- **`browserbase-pp-cli projects get`** - Get a Project
- **`browserbase-pp-cli projects list`** - List Projects

### sessions

Manage sessions

- **`browserbase-pp-cli sessions create`** - Create a Session
- **`browserbase-pp-cli sessions get`** - Get a Session
- **`browserbase-pp-cli sessions list`** - List Sessions
- **`browserbase-pp-cli sessions update`** - Update a Session

### websearch

Manage websearch

- **`browserbase-pp-cli websearch`** - Perform a web search and return structured results.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`browserbase-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`browserbase-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`browserbase-pp-cli learnings list`** - Inspect taught rows
- **`browserbase-pp-cli learnings forget <query>`** - Undo a teach
- **`browserbase-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`browserbase-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`browserbase-pp-cli teach-pattern`** - Install a query/resource template up front
- **`browserbase-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `BROWSERBASE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `browserbase-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
browserbase-pp-cli agents list

# JSON for scripting and agents
browserbase-pp-cli agents list --json
# Filter to specific fields
browserbase-pp-cli agents list --json --select agentId,createdAt,name

# Dry run — show the request without sending
browserbase-pp-cli agents list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
browserbase-pp-cli agents list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
browserbase-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `browserbase-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/browserbase-pp-cli/config.toml`; `--home`, `BROWSERBASE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BROWSERBASE_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `browserbase-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `browserbase-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BROWSERBASE_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized / Missing x-bb-api-key header** — Set BROWSERBASE_API_KEY to your key from browserbase.com/settings.
- **429 rate limited on fetch/search** — Use `fetch batch` for paced fetching, or back off — fetch is 5 req/sec, search 2 req/sec.
- **keepAlive sessions keep running after my script exits** — Use `sessions run` for guaranteed release, or `sessions orphans --stop` to clean up stragglers.
- **Session timed out unexpectedly** — Pass `--timeout` (60-21600s) to `sessions create`; the project default may be shorter than your job.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**browse**](https://github.com/browserbase/stagehand) — TypeScript
- [**sdk-node**](https://github.com/browserbase/sdk-node) — TypeScript
- [**sdk-python**](https://github.com/browserbase/sdk-python) — Python
- [**mcp-server-browserbase**](https://github.com/browserbase/mcp-server-browserbase) — TypeScript
- [**steel-cli**](https://github.com/steel-dev/steel-browser) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
