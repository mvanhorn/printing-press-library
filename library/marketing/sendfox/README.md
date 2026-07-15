# SendFox CLI

Operate SendFox contacts, lists, campaign reads, audience hygiene, CSV reconciliation, launch checklists, signup-form handoffs, webhook setup packets, and public API capability checks from the terminal.

## Install

The recommended path installs both the `sendfox-pp-cli` binary and the `pp-sendfox` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sendfox
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sendfox --agent claude-code
npx -y @mvanhorn/printing-press-library install sendfox --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendfox-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sendfox --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sendfox --force
```

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install sendfox --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendfox-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SENDFOX_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/sendfox/cmd/sendfox-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sendfox": {
      "command": "sendfox-pp-mcp",
      "env": {
        "SENDFOX_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Create a SendFox personal access token at https://sendfox.com/account/oauth, then store it:

```bash
sendfox-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export SENDFOX_API_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
sendfox-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
sendfox-pp-cli workflow account-snapshot --agent
```

## Usage

Run `sendfox-pp-cli --help` for the full command reference and flag list.

## Commands

### capabilities

Documented public API support matrix

- **`sendfox-pp-cli capabilities`** - Show which SendFox resources support list/get/create/update/delete and where dashboard handoffs are required

### campaigns

Campaign reads

- **`sendfox-pp-cli campaigns get`** - Get campaign by ID
- **`sendfox-pp-cli campaigns list`** - List campaigns

### contacts

Contacts and subscription state

- **`sendfox-pp-cli contacts create`** - Create a contact
- **`sendfox-pp-cli contacts get`** - Get a contact by ID
- **`sendfox-pp-cli contacts list`** - List contacts or find a contact by email
- **`sendfox-pp-cli contacts onboard`** - Create a contact and attach list memberships in one automation-aware flow
- **`sendfox-pp-cli contacts import-csv`** - Bulk-create contacts from CSV behind a dry-run/--yes safety gate
- **`sendfox-pp-cli contacts audit-csv`** - Validate subscriber CSVs for invalid and duplicate emails before any mutation
- **`sendfox-pp-cli contacts reconcile-csv`** - Compare a CSV against live contacts and emit create/skip actions

### forms

Generate SendFox integration assets

- **`sendfox-pp-cli forms generate`** - Generate a self-contained HTML signup form and server-proxy handoff

### lists

Contact lists

- **`sendfox-pp-cli lists create`** - Create a contact list
- **`sendfox-pp-cli lists get`** - Get a contact list by ID
- **`sendfox-pp-cli lists list-lists`** - List contact lists

### workflow

Compound SendFox workflows for agents

- **`sendfox-pp-cli workflow account-snapshot`** - Summarize account, list, contact, and campaign state
- **`sendfox-pp-cli workflow audience-map`** - Map contacts to lists and surface segmentation gaps
- **`sendfox-pp-cli workflow campaign-digest`** - Summarize campaign count, status mix, and recency
- **`sendfox-pp-cli workflow hygiene-report`** - Find duplicate emails, invalid emails, status mix, and list-membership gaps
- **`sendfox-pp-cli workflow launch-plan`** - Generate a safe SendFox list-launch checklist and exact next CLI/dashboard steps

### webhooks

Generate SendFox webhook/dashboard handoffs

- **`sendfox-pp-cli webhooks handoff`** - Generate dashboard setup and handler-contract packets for SendFox webhook receivers

### me

Manage me

- **`sendfox-pp-cli me`** - Get authenticated user

### unsubscribe

Manage unsubscribe

- **`sendfox-pp-cli unsubscribe`** - Unsubscribe a contact by email


## Unique Features

- **`capabilities`** — machine-readable API support matrix, so agents do not invent unsupported SendFox campaign/webhook write calls.
- **`workflow account-snapshot`** — one read-only SendFox operating packet across account, lists, contacts, and campaigns. Use this before planning campaigns or auditing account state.
- **`workflow audience-map`** — list membership map with contacts that are not attached to any list, useful for cleanup and automation-trigger checks.
- **`workflow campaign-digest`** — status-count and recent-campaign digest without hand-rolling campaign list parsing.
- **`workflow hygiene-report`** — live hygiene report for duplicate/invalid emails, contact status mix, and contacts without lists.
- **`workflow launch-plan`** — safe launch checklist that validates the list, emits exact next commands, and explicitly keeps campaign creation/sending in the dashboard because the public docs expose campaign reads only.
- **`contacts audit-csv`** — preflight CSV validation for duplicate and invalid subscriber emails before any SendFox mutation.
- **`contacts reconcile-csv`** — compares a CSV to live contacts, then returns create/skip actions for agent review.
- **`contacts onboard`** — one command to create a subscriber and attach list IDs, with `--dry-run --agent` showing the exact request that may trigger list automations.
- **`contacts import-csv`** — guarded bulk importer for `email,first_name,last_name` CSVs; live runs require `--yes` after dry-run review.
- **`forms generate`** — creates an embeddable HTML signup form plus explicit server-proxy note so browser code never leaks the bearer token.
- **`webhooks handoff`** — dashboard setup packet and receiver contract for webhook installs, without pretending there is public webhook CRUD.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sendfox-pp-cli campaigns list

# JSON for scripting and agents
sendfox-pp-cli campaigns list --json

# Filter to specific fields
sendfox-pp-cli campaigns list --json --select id,name,status

# Dry run — show the request without sending
sendfox-pp-cli campaigns list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sendfox-pp-cli campaigns list --agent
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
sendfox-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sendfox-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SENDFOX_API_TOKEN` | per_call | Yes | Preferred SendFox personal access token env var. |
| `SENDFOX_BEARER_AUTH` | per_call | No | Backward-compatible alias for `SENDFOX_API_TOKEN`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sendfox-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sendfox-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SENDFOX_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
