# Paperclip CLI

REST API for the Paperclip AI agent management platform

Created by [@veltri-23](https://github.com/veltri-23) (Hunter Veltri).

## Install

The recommended path installs both the `paperclip-pp-cli` binary and the `pp-paperclip` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install paperclip
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install paperclip --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install paperclip --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install paperclip --agent claude-code
npx -y @mvanhorn/printing-press-library install paperclip --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/paperclip-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install paperclip --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-paperclip --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-paperclip --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install paperclip --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/paperclip-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "paperclip": {
      "command": "paperclip-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Verify Setup

Configure one of Paperclip's supported authentication modes first:

```bash
# Browser-session authentication
export PAPERCLIP_SESSION_COOKIE="<session-cookie>"

# Or a board API key
export PAPERCLIP_API_KEY="<board-api-key>"

# Or an agent bearer token
export PAPERCLIP_AGENT_TOKEN="<agent-token>"
```

The CLI auto-detects the mode from the credential. Set `PAPERCLIP_AUTH_MODE` to
`board-session`, `board-api-key`, `agent-bearer`, or `none` to choose explicitly.

```bash
paperclip-pp-cli doctor
```

This checks your configuration.

### 3. Try Your First Command

```bash
paperclip-pp-cli adapters list
```

## Usage

Run `paperclip-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `PAPERCLIP_CONFIG_DIR`, `PAPERCLIP_DATA_DIR`, `PAPERCLIP_STATE_DIR`, or `PAPERCLIP_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `PAPERCLIP_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export PAPERCLIP_HOME=/srv/paperclip
paperclip-pp-cli doctor
```

Under `PAPERCLIP_HOME=/srv/paperclip`, the four dirs resolve to `/srv/paperclip/config`, `/srv/paperclip/data`, `/srv/paperclip/state`, and `/srv/paperclip/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "paperclip": {
      "command": "paperclip-pp-mcp",
      "env": {
        "PAPERCLIP_HOME": "/srv/paperclip"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `PAPERCLIP_DATA_DIR` overrides an explicit `--home` for that kind. Use `PAPERCLIP_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `PAPERCLIP_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `paperclip-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### adapters

Manage adapters

- **`paperclip-pp-cli adapters create`** - Install an adapter
- **`paperclip-pp-cli adapters delete`** - Delete an adapter
- **`paperclip-pp-cli adapters get`** - Get adapter registration details
- **`paperclip-pp-cli adapters list`** - List all adapters
- **`paperclip-pp-cli adapters update`** - Enable or disable an adapter

### admin

Manage admin

- **`paperclip-pp-cli admin create`** - Demote a user from instance admin
- **`paperclip-pp-cli admin create-users`** - Promote a user to instance admin
- **`paperclip-pp-cli admin get`** - Get company access for a user (admin)
- **`paperclip-pp-cli admin list`** - List all users (admin)
- **`paperclip-pp-cli admin update`** - Set company access for a user (admin)

### agents

Manage agents

- **`paperclip-pp-cli agents delete`** - Delete an agent
- **`paperclip-pp-cli agents get`** - Get an agent
- **`paperclip-pp-cli agents list`** - Get the current agent
- **`paperclip-pp-cli agents list-me`** - Get current agent inbox (lite)
- **`paperclip-pp-cli agents list-me-2`** - Get current agent assigned inbox items
- **`paperclip-pp-cli agents update`** - Update an agent

### approvals

Manage approvals

- **`paperclip-pp-cli approvals <id>`** - Get an approval

### assets

Manage assets


### attachments

Manage attachments

- **`paperclip-pp-cli attachments <attachmentId>`** - Delete an attachment

### auth

Manage auth

- **`paperclip-pp-cli auth list`** - Get current session
- **`paperclip-pp-cli auth list-profile`** - Get current user profile
- **`paperclip-pp-cli auth update`** - Update current user profile

### board

Manage board

- **`paperclip-pp-cli board`** - Stream a board-level chat response (requires enableConferenceRoomChat)

### board-api-keys

Manage board api keys

- **`paperclip-pp-cli board-api-keys create`** - Create a named board API key
- **`paperclip-pp-cli board-api-keys delete`** - Revoke a board API key
- **`paperclip-pp-cli board-api-keys list`** - List board API keys

### board-claim

Manage board claim

- **`paperclip-pp-cli board-claim <token>`** - Get board claim details by token

### bootstrap

Manage bootstrap

- **`paperclip-pp-cli bootstrap`** - Claim first instance admin from a browser session

### cli-auth

Manage cli auth

- **`paperclip-pp-cli cli-auth create`** - Create a CLI auth challenge
- **`paperclip-pp-cli cli-auth create-cliauth`** - Revoke current CLI auth session
- **`paperclip-pp-cli cli-auth create-cliauth-2`** - Approve a CLI auth challenge
- **`paperclip-pp-cli cli-auth create-cliauth-3`** - Cancel a CLI auth challenge
- **`paperclip-pp-cli cli-auth get`** - Get a CLI auth challenge
- **`paperclip-pp-cli cli-auth list`** - Get current CLI auth session

### cloud-upstreams

Manage cloud upstreams

- **`paperclip-pp-cli cloud-upstreams create`** - Finish a cloud upstream connection
- **`paperclip-pp-cli cloud-upstreams create-cloudupstreams`** - Start a cloud upstream connection
- **`paperclip-pp-cli cloud-upstreams list`** - List cloud upstream connections

### companies

Manage companies

- **`paperclip-pp-cli companies create`** - Create a company
- **`paperclip-pp-cli companies create-import`** - Apply a company import (legacy route)
- **`paperclip-pp-cli companies create-import-2`** - Preview a company import (legacy route)
- **`paperclip-pp-cli companies delete`** - Delete a company
- **`paperclip-pp-cli companies get`** - Get a company
- **`paperclip-pp-cli companies get-import`** - Get company import job status
- **`paperclip-pp-cli companies list`** - List companies
- **`paperclip-pp-cli companies list-issues`** - Legacy — returns error directing to correct issues path
- **`paperclip-pp-cli companies list-stats`** - Company stats
- **`paperclip-pp-cli companies update`** - Update a company

### environment-custom-image-setup-sessions

Manage environment custom image setup sessions

- **`paperclip-pp-cli environment-custom-image-setup-sessions <sessionId>`** - Get and refresh an environment customImage setup session

### environment-leases

Manage environment leases

- **`paperclip-pp-cli environment-leases <leaseId>`** - Get an environment lease

### environments

Manage environments

- **`paperclip-pp-cli environments delete`** - Delete an environment
- **`paperclip-pp-cli environments get`** - Get an environment
- **`paperclip-pp-cli environments update`** - Update an environment

### execution-workspaces

Manage execution workspaces

- **`paperclip-pp-cli execution-workspaces get`** - Get an execution workspace
- **`paperclip-pp-cli execution-workspaces update`** - Update an execution workspace

### feedback-traces

Manage feedback traces

- **`paperclip-pp-cli feedback-traces <traceId>`** - Get a feedback trace

### goals

Manage goals

- **`paperclip-pp-cli goals delete`** - Delete a goal
- **`paperclip-pp-cli goals get`** - Get a goal
- **`paperclip-pp-cli goals update`** - Update a goal

### health

Manage health

- **`paperclip-pp-cli health create`** - Request a managed dev-server restart
- **`paperclip-pp-cli health list`** - Health check

### heartbeat-runs

Manage heartbeat runs

- **`paperclip-pp-cli heartbeat-runs <runId>`** - Get a heartbeat run

### instance

Manage instance

- **`paperclip-pp-cli instance create`** - Trigger a database backup
- **`paperclip-pp-cli instance create-settings`** - Preview issue graph liveness auto-recovery
- **`paperclip-pp-cli instance create-settings-2`** - Run issue graph liveness auto-recovery
- **`paperclip-pp-cli instance list`** - List scheduler heartbeats
- **`paperclip-pp-cli instance list-settings`** - Get instance settings
- **`paperclip-pp-cli instance list-settings-2`** - Get experimental instance settings
- **`paperclip-pp-cli instance list-settings-3`** - Get general instance settings
- **`paperclip-pp-cli instance update`** - Update instance settings
- **`paperclip-pp-cli instance update-settings`** - Update experimental instance settings
- **`paperclip-pp-cli instance update-settings-2`** - Update general instance settings

### invites

Manage invites

- **`paperclip-pp-cli invites <token>`** - Get an invite by token

### issues

Manage issues

- **`paperclip-pp-cli issues delete`** - Delete an issue
- **`paperclip-pp-cli issues get`** - Get an issue
- **`paperclip-pp-cli issues list`** - Legacy — returns error directing to /api/companies/{companyId}/issues
- **`paperclip-pp-cli issues update`** - Update an issue

### join-requests

Manage join requests


### labels

Manage labels

- **`paperclip-pp-cli labels <labelId>`** - Delete a label

### llms

Manage llms

- **`paperclip-pp-cli llms get`** - Get agent configuration for a specific adapter type
- **`paperclip-pp-cli llms list`** - Get agent configuration as plain text (for LLM context)
- **`paperclip-pp-cli llms list-agenticonstxt`** - Get agent icon names as plain text

### openapi-json

Manage openapi json

- **`paperclip-pp-cli openapi-json`** - Get the generated OpenAPI document

### plugins

Manage plugins

- **`paperclip-pp-cli plugins create`** - Install a plugin
- **`paperclip-pp-cli plugins create-tools`** - Execute a plugin tool
- **`paperclip-pp-cli plugins delete`** - Delete a plugin
- **`paperclip-pp-cli plugins get`** - Get a plugin
- **`paperclip-pp-cli plugins list`** - List installed plugins
- **`paperclip-pp-cli plugins list-examples`** - List example plugins
- **`paperclip-pp-cli plugins list-tools`** - List plugin tools
- **`paperclip-pp-cli plugins list-uicontributions`** - List plugin UI contributions

### projects

Manage projects

- **`paperclip-pp-cli projects delete`** - Delete a project
- **`paperclip-pp-cli projects get`** - Get a project
- **`paperclip-pp-cli projects update`** - Update a project

### routine-triggers

Manage routine triggers

- **`paperclip-pp-cli routine-triggers create`** - Fire a public routine trigger
- **`paperclip-pp-cli routine-triggers delete`** - Delete a routine trigger
- **`paperclip-pp-cli routine-triggers update`** - Update a routine trigger

### routines

Manage routines

- **`paperclip-pp-cli routines get`** - Get a routine
- **`paperclip-pp-cli routines update`** - Update a routine

### secret-provider-configs

Manage secret provider configs

- **`paperclip-pp-cli secret-provider-configs delete`** - Delete a secret provider configuration
- **`paperclip-pp-cli secret-provider-configs get`** - Get a secret provider configuration
- **`paperclip-pp-cli secret-provider-configs update`** - Update a secret provider configuration

### secrets

Manage secrets

- **`paperclip-pp-cli secrets delete`** - Delete a secret
- **`paperclip-pp-cli secrets update`** - Update a secret

### sidebar-preferences

Manage sidebar preferences

- **`paperclip-pp-cli sidebar-preferences list`** - Get current user sidebar preferences
- **`paperclip-pp-cli sidebar-preferences update`** - Update current user sidebar preferences

### skills

Manage skills

- **`paperclip-pp-cli skills get`** - Get a skill by name
- **`paperclip-pp-cli skills get-catalog`** - Get a catalog skill
- **`paperclip-pp-cli skills get-catalog-2`** - List catalog skill files
- **`paperclip-pp-cli skills list`** - List available skills
- **`paperclip-pp-cli skills list-catalog`** - List catalog skills
- **`paperclip-pp-cli skills list-index`** - Get skills index

### teams

Manage teams

- **`paperclip-pp-cli teams get`** - Get catalog team
- **`paperclip-pp-cli teams get-catalog`** - Get catalog team file
- **`paperclip-pp-cli teams list`** - List catalog teams

### work-products

Manage work products

- **`paperclip-pp-cli work-products delete`** - Delete a work product
- **`paperclip-pp-cli work-products update`** - Update a work product

### workspace-operations

Manage workspace operations



### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`paperclip-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`paperclip-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`paperclip-pp-cli learnings list`** - Inspect taught rows
- **`paperclip-pp-cli learnings forget <query>`** - Undo a teach
- **`paperclip-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`paperclip-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`paperclip-pp-cli teach-pattern`** - Install a query/resource template up front
- **`paperclip-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `PAPERCLIP_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `paperclip-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
paperclip-pp-cli adapters list

# JSON for scripting and agents
paperclip-pp-cli adapters list --json

# Filter to specific fields
paperclip-pp-cli adapters list --json --select id,name,status

# Dry run — show the request without sending
paperclip-pp-cli adapters list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
paperclip-pp-cli adapters list --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
paperclip-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `paperclip-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/paperclip-pp-cli/config.toml`; `--home`, `PAPERCLIP_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
