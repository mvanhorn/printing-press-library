# v0 CLI

**Generate and iterate on web apps with v0 from the terminal: create chats from prompts, stream agent output, sync history to a local SQLite mirror, and track credit spend across models.**

The v0 API v2 CLI with offline search, streaming capture, and credit-spend analytics — no other tool tracks where your v0 credits go.

## Install

The recommended path installs both the `v0-pp-cli` binary and the `pp-v0` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install v0
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install v0 --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install v0 --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install v0 --agent claude-code
npx -y @mvanhorn/printing-press-library install v0 --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/v0/cmd/v0-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/v0-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install v0 --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-v0 --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-v0 --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install v0 --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/v0-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `V0_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai/v0/cmd/v0-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "v0": {
      "command": "v0-pp-mcp",
      "env": {
        "V0_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Requires a v0 API key (create one at https://v0.app/settings/keys). Set V0_API_KEY in your environment, or use auth set-token. The CLI sends Authorization: Bearer <key>.

## Quick Start

```bash
# Verify the CLI and API key are configured correctly
v0-pp-cli doctor --dry-run

# List your most recent chats
v0-pp-cli chats list --limit 5

# Get chat details as JSON
v0-pp-cli chats get ft7dqhYEX8n --json

# Create a chat and stream the generation live
v0-pp-cli chats stream "Create a landing page" --model v0-mini

# Sync chats and messages into the local mirror
v0-pp-cli sync --resources chats,messages

# Search your synced history offline
v0-pp-cli search "dashboard" --json

# See where your v0 credits went this week
v0-pp-cli spend --since 7d --by chat

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cost intelligence
- **`spend`** — Aggregate v0 credit cost and token usage from the synced message mirror, grouped by chat, day, or model.

  _Use this when an agent needs to know where v0 credits went, which chats are expensive, or a daily burn rate._

  ```bash
  v0-pp-cli spend --since 7d --by chat --json
  ```

### Streaming
- **`chats stream`** — Create a chat and stream the SSE response live, rendering each event and recording model attribution for spend analytics.

  _Use this when an agent needs to see generation progress live or capture the event stream for later analysis._

  ```bash
  v0-pp-cli chats stream "Create a kanban dashboard" --model v0-pro
  ```
- **`messages tail`** — Poll a chat until the newest assistant message finishes, with --follow for continuous watching.

  _Use this when an agent kicked off an async generation and needs to wait for it deterministically._

  ```bash
  v0-pp-cli messages tail ft7dqhYEX8n --interval 3s --timeout 10m
  ```

### Files
- **`chats files`** — Render a chat's generated source files as an indented directory tree.

  _Use this when an agent needs to understand a generated app's layout before editing._

  ```bash
  v0-pp-cli chats files ft7dqhYEX8n --tree
  ```
- **`chats preview`** — Print only the live preview URL for a chat, ready for embedding or scripting.

  _Use this when an agent needs the preview URL for CI, screenshots, or quick checks._

  ```bash
  v0-pp-cli chats preview ft7dqhYEX8n --url
  ```

### Offline
- **`search`** — Full-text search over synced chats and messages without hitting the API.

  _Use this when an agent needs to find a past chat or message without paging the API._

  ```bash
  v0-pp-cli search "kanban" --json
  ```
- **`sync`** — Cursor-paginated sync of chats and messages into a local SQLite mirror for offline search and spend analytics.

  _Run once before offline search or spend commands._

  ```bash
  v0-pp-cli sync --resources chats,messages
  ```

### Ops
- **`doctor`** — Validate V0_API_KEY and API reachability with a live check.

  _Use this when a script 401s and you need to know why._

  ```bash
  v0-pp-cli doctor
  ```

## Recipes

### Create a chat and wait for it

```bash
v0-pp-cli chats create --message "A minimal landing page" --title Landing --privacy private --json
```

Blocking create returns the chat plus token/credit usage once the model finishes.

### Stream a generation and record the model

```bash
v0-pp-cli chats stream "A pricing page" --model v0-pro --privacy private
```

Streams SSE events live and records model attribution so spend --by model works.

### Send a follow-up asynchronously and tail it

```bash
v0-pp-cli messages send-async ft7dqhYEX8n --message "Add dark mode" --json
```

Async send returns a messageId; poll it with messages tail until finishReason is set.

### Inspect generated files as a tree

```bash
v0-pp-cli chats files ft7dqhYEX8n --tree
```

Renders the generated app's file layout without downloading the archive.

### Track weekly credit spend by chat

```bash
v0-pp-cli spend --since 7d --by chat --json
```

After sync, aggregates usage.creditsCost and tokens from the local mirror.

### Embed a live preview

```bash
v0-pp-cli chats preview ft7dqhYEX8n --url
```

Prints just the preview URL for embedding or scripting.

## Usage

Run `v0-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `V0_CONFIG_DIR`, `V0_DATA_DIR`, `V0_STATE_DIR`, or `V0_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `V0_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export V0_HOME=/srv/v0
v0-pp-cli doctor
```

Under `V0_HOME=/srv/v0`, the four dirs resolve to `/srv/v0/config`, `/srv/v0/data`, `/srv/v0/state`, and `/srv/v0/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "v0": {
      "command": "v0-pp-mcp",
      "env": {
        "V0_HOME": "/srv/v0"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `V0_DATA_DIR` overrides an explicit `--home` for that kind. Use `V0_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `V0_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `v0-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### chats

Generate and manage apps from prompts

- **`v0-pp-cli chats connect-status`** - Get the setup status of a Vercel Connect integration
- **`v0-pp-cli chats create`** - Create a chat from a prompt (blocks until the model response is complete)
- **`v0-pp-cli chats create-async`** - Create a chat asynchronously; poll the returned messageId for completion
- **`v0-pp-cli chats create-from-files`** - Create a chat from source files
- **`v0-pp-cli chats create-from-repo`** - Create a chat from a GitHub repository
- **`v0-pp-cli chats create-from-zip`** - Create a chat from a ZIP archive URL
- **`v0-pp-cli chats create-stream`** - Create a chat and stream the model response as Server-Sent Events
- **`v0-pp-cli chats create-vercel-project`** - Create a Vercel project for a chat
- **`v0-pp-cli chats delete`** - Delete a chat permanently
- **`v0-pp-cli chats deploy`** - Deploy a chat to Vercel
- **`v0-pp-cli chats download-files`** - Download all chat files as an archive
- **`v0-pp-cli chats duplicate`** - Duplicate a chat
- **`v0-pp-cli chats files`** - Get all source files in a chat
- **`v0-pp-cli chats get`** - Get a chat by ID
- **`v0-pp-cli chats list`** - List chats accessible to the authenticated user
- **`v0-pp-cli chats preview`** - Get the preview URL and short-lived access token for a chat
- **`v0-pp-cli chats restore-message`** - Restore files from a previous assistant message
- **`v0-pp-cli chats resume-stream`** - Reconnect to an active chat stream
- **`v0-pp-cli chats update`** - Update a chat title, privacy, or metadata
- **`v0-pp-cli chats update-files`** - Create, update, or delete files in a chat

### hooks

Manage webhooks that listen for chat and message events

- **`v0-pp-cli hooks create`** - Create a webhook subscribed to chat and message events
- **`v0-pp-cli hooks delete`** - Delete a webhook
- **`v0-pp-cli hooks get`** - Get a webhook by ID
- **`v0-pp-cli hooks list`** - List all webhooks in the workspace
- **`v0-pp-cli hooks update`** - Update a webhook configuration

### mcp-servers

Manage MCP server connections for chats (max 10 per user)

- **`v0-pp-cli mcp-servers create`** - Register a new MCP server
- **`v0-pp-cli mcp-servers delete`** - Delete an MCP server
- **`v0-pp-cli mcp-servers get`** - Get an MCP server by ID
- **`v0-pp-cli mcp-servers list`** - List MCP servers configured for the account
- **`v0-pp-cli mcp-servers update`** - Update an MCP server configuration

### messages

Send, list, and manage chat messages

- **`v0-pp-cli messages get`** - Get a single message
- **`v0-pp-cli messages list`** - List messages in a chat, newest first
- **`v0-pp-cli messages resolve-task`** - Resolve a chat blocked waiting for user input
- **`v0-pp-cli messages resolve-task-async`** - Resolve a blocked chat asynchronously
- **`v0-pp-cli messages resolve-task-stream`** - Resolve a blocked chat and stream the response
- **`v0-pp-cli messages send`** - Send a message and wait for the model response
- **`v0-pp-cli messages send-async`** - Send a message asynchronously; poll the returned messageId for completion
- **`v0-pp-cli messages send-stream`** - Send a message and stream the response as Server-Sent Events
- **`v0-pp-cli messages stop`** - Stop an in-flight message generation

### settings

Manage workspace settings

- **`v0-pp-cli settings preview-hosts`** - Get trusted hostname patterns allowed to embed previews
- **`v0-pp-cli settings set-preview-hosts`** - Set the complete list of trusted preview hosts


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`v0-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`v0-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`v0-pp-cli learnings list`** - Inspect taught rows
- **`v0-pp-cli learnings forget <query>`** - Undo a teach
- **`v0-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`v0-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`v0-pp-cli teach-pattern`** - Install a query/resource template up front
- **`v0-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `V0_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `v0-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
v0-pp-cli chats list

# JSON for scripting and agents
v0-pp-cli chats list --json

# Filter to specific fields
v0-pp-cli chats list --json --select id,name,status

# Dry run — show the request without sending
v0-pp-cli chats list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
v0-pp-cli chats list --agent
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
v0-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `v0-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/v0/config.toml`; `--home`, `V0_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `V0_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `v0-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `v0-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $V0_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — V0_API_KEY is missing, invalid, or expired; create a key at https://v0.app/settings/keys
- **422 "API v2 does not support this chat"** — The chat was created through the v1 API; v2 only serves v2-created chats
- **Script failures with unclear cause** — Run doctor to verify auth and connectivity before debugging

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**v0 npm SDK**](https://www.npmjs.com/package/v0) — TypeScript (510 stars)
- [**v0-cli (community)**](https://www.npmjs.com/package/v0-cli) — JavaScript
- [**v0 MCP server**](https://v0.app/docs/api/v2/guides/mcp-server) — TypeScript
- [**v0 web UI**](https://v0.app) — Web

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
