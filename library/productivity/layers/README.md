# Layers Digital CLI

**School announcements, agenda events, account context, and a private local search surface in one agent-ready CLI.**

The CLI unifies the Layers portal and its embedded announcements and agenda services behind stable JSON commands. App-scoped sessions are derived only in process memory, while sync and search support repeated agent workflows without persisting credentials.

Learn more at [Layers Digital](https://api.layers.digital).

## Install

The recommended path installs both the `layers-pp-cli` binary and the `pp-layers` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install layers
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install layers --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install layers --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install layers --agent claude-code
npx -y @mvanhorn/printing-press-library install layers --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/layers/cmd/layers-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/layers-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install layers --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-layers --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-layers --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install layers --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/layers-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `LAYERS_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/layers/cmd/layers-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "layers": {
      "command": "layers-pp-mcp",
      "env": {
        "LAYERS_TOKEN": "<your-key>",
        "LAYERS_COMMUNITY_ID": "<your-community-slug>"
      }
    }
  }
}
```

</details>

## Authentication

Set LAYERS_TOKEN and LAYERS_COMMUNITY_ID in the process environment. `LAYERS_TOKEN` accepts either the raw token or a full `Bearer <token>` header value; the CLI normalizes it to one outbound bearer header. The CLI derives embedded-app sessions in memory and never prints or persists them.

## Quick Start

```bash
# Verify local setup without making a network request.
layers-pp-cli doctor --dry-run

# Confirm the selected school context.
layers-pp-cli context --agent

# Read the first page of announcements through an in-memory app session.
layers-pp-cli post-delivery --batch-size 20 --filters '{}' --agent

# Synchronize agenda events for a localized month slug.
layers-pp-cli view --months '["august-2026"]' --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Search previously synchronized school data across Layers services from one local, pipeable index.

  _Use this when an agent needs a fast cross-service lookup without repeatedly downloading verbose portal responses._

  ```bash
  layers-pp-cli search "local" --data-source local --agent
  ```

## Recipes

### Compact announcements

```bash
layers-pp-cli post-delivery --batch-size 20 --filters '{}' --agent --select hits._id,hits.title,next
```

Read a page while limiting the JSON fields retained by the agent.

### Agenda month

```bash
layers-pp-cli view --months '["august-2026"]' --agent
```

Fetch one localized calendar month without mutating the portal.

### Offline school search

```bash
layers-pp-cli search "local" --data-source local --agent
```

Search the private SQLite snapshot without a network request.

## Usage

Run `layers-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `LAYERS_CONFIG_DIR`, `LAYERS_DATA_DIR`, `LAYERS_STATE_DIR`, or `LAYERS_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `LAYERS_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export LAYERS_HOME=/srv/layers
layers-pp-cli doctor
```

Under `LAYERS_HOME=/srv/layers`, the four dirs resolve to `/srv/layers/config`, `/srv/layers/data`, `/srv/layers/state`, and `/srv/layers/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "layers": {
      "command": "layers-pp-mcp",
      "env": {
        "LAYERS_HOME": "/srv/layers"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `LAYERS_DATA_DIR` overrides an explicit `--home` for that kind. Use `LAYERS_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `LAYERS_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `layers-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### communities

Operations on search

- **`layers-pp-cli communities`** - GET /v2/communities/search

### context

Operations on context

- **`layers-pp-cli context`** - GET /v1/context

### home

Operations on discover

- **`layers-pp-cli home list-discover`** - GET /v1/home/discover
- **`layers-pp-cli home list-layers-agenda`** - GET /v1/home/getHomeInfo/layers-agenda
- **`layers-pp-cli home list-layers-atendimentos`** - GET /v1/home/getHomeInfo/layers-atendimentos
- **`layers-pp-cli home list-layers-comunicados`** - GET /v1/home/getHomeInfo/layers-comunicados

### me

Operations on me

- **`layers-pp-cli me`** - GET /v1/me

### media

Operations on icon

- **`layers-pp-cli media list-icon`** - GET /v1/media/app/layers-comunicados/icon
- **`layers-pp-cli media list-icon-2`** - GET /v1/media/app/layers-agenda/icon
- **`layers-pp-cli media list-icon-3`** - GET /v1/media/app/layers-atendimentos/icon

### notification-badge

Operations on notification-badge

- **`layers-pp-cli notification-badge`** - GET /v2/notification-badge

### post-delivery

Operations on paginate

- **`layers-pp-cli post-delivery`** - POST /api/v2/post-delivery/paginate

### preferences

Operations on preferences

- **`layers-pp-cli preferences list-preferences`** - GET /v2/preferences
- **`layers-pp-cli preferences update-preferences`** - PUT /v2/preferences

### sso

Operations on me

- **`layers-pp-cli sso`** - GET /v1/sso/account/me

### view

Operations on sync

- **`layers-pp-cli view`** - POST /v1/view/events/sync


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`layers-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`layers-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`layers-pp-cli learnings list`** - Inspect taught rows
- **`layers-pp-cli learnings forget <query>`** - Undo a teach
- **`layers-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`layers-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`layers-pp-cli teach-pattern`** - Install a query/resource template up front
- **`layers-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `LAYERS_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `layers-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
layers-pp-cli communities

# JSON for scripting and agents
layers-pp-cli communities --json

# Filter to specific fields
layers-pp-cli communities --json --select id,name,status

# Dry run — show the request without sending
layers-pp-cli communities --dry-run

# Agent mode — JSON + compact + no prompts in one flag
layers-pp-cli communities --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
layers-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `layers-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/layers-pp-cli/config.toml`; `--home`, `LAYERS_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `LAYERS_TOKEN` | per_call | Yes | Set to your API credential. Raw token preferred; full `Bearer <token>` also accepted. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `layers-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `layers-pp-cli doctor` to check credentials
- Verify the environment variable is present without printing it: `test -n "${LAYERS_TOKEN:-}" && echo set || echo missing`
- If you copied the browser `Authorization` header verbatim, leaving the `Bearer ` prefix in `LAYERS_TOKEN` is supported.
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **The context command says community-id is required.** — Set LAYERS_COMMUNITY_ID to the active school community slug and retry.
- **An embedded-app command returns HTTP 400.** — Refresh LAYERS_TOKEN from a current signed-in browser session; the CLI will derive a fresh app session in memory.
