# Sybill CLI

**The first CLI for Sybill: every conversation, deal, and account in your terminal, plus a local store that answers cross-entity questions the web UI can't.**

Sybill records sales calls and generates AI summaries, deal briefs, and crmAutofill suggestions. This CLI pulls all of it into a local SQLite store you can query offline, then adds the joins the entity-by-entity API can't do: deals gone dark, a weekly call digest grouped by deal, pending crmAutofill diffs, and full-text transcript search.

Learn more at [Sybill](https://www.sybill.ai).

Created by [@riccardovandra](https://github.com/riccardovandra).

## Install

The recommended path installs both the `sybill-pp-cli` binary and the `pp-sybill` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sybill
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sybill --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sybill --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sybill --agent claude-code
npx -y @mvanhorn/printing-press-library install sybill --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sybill-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sybill --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sybill --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-sybill skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-sybill. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sybill-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SYBILL_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sybill": {
      "command": "sybill-pp-mcp",
      "env": {
        "SYBILL_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Create an API key in the Sybill dashboard under Settings > Integrations > API Keys (it starts with sk_live_), then set SYBILL_API_KEY. Run doctor to confirm the key is valid and the API is reachable. Read commands need a key with the read scope; ingest commands need the ingest scope (both are set on the key in the dashboard).

## Quick Start

```bash
# Confirm the API key is valid and see which scopes it carries
sybill-pp-cli doctor

# Pull conversations, deals, and accounts into the local store
sybill-pp-cli sync

# Find open deals that have gone quiet
sybill-pp-cli deals dark --days 14

# Build this week's call digest grouped by deal
sybill-pp-cli digest --since 7d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-entity pipeline intelligence
- **`deals dark`** — List open deals with no call activity in the last N days, so nothing stalls silently.

  _Reach for this when an agent needs the re-engagement list: which open opportunities have gone quiet._

  ```bash
  sybill-pp-cli deals dark --days 14 --agent
  ```
- **`digest`** — Pull every call in a window, grouped by deal, with next steps per deal (summaries appear when conversation detail is synced).

  _Use this to generate a Monday pipeline review without clicking through every call._

  ```bash
  sybill-pp-cli digest --since 7d --agent
  ```
- **`account rollup`** — One offline view per account: call count, open deals by stage, contacts, and last activity.

  _Use this to prep a renewal or expansion conversation with full account context._

  ```bash
  sybill-pp-cli account rollup acc_456 --agent
  ```
- **`activity`** — Per-rep breakdown: calls made, deals touched, and deals gone dark over a window.

  _Use this for manager-side coaching prep and pipeline-coverage checks._

  ```bash
  sybill-pp-cli activity --by owner --since 7d --agent
  ```

### Sybill-specific signal
- **`crm-autofill`** — Show the AI-suggested CRM field updates Sybill generated, as a reviewable field-by-field diff.

  _Reach for this before pushing CRM updates, to review what Sybill wants to change._

  ```bash
  sybill-pp-cli crm-autofill --deal deal_123 --agent
  ```
- **`patterns`** — Count and locate transcript mentions of a term, grouped by deal and stage.

  _Reach for this to find where competitor mentions, pricing objections, or legal flags cluster._

  ```bash
  sybill-pp-cli patterns --term pricing --agent
  ```

## Recipes


### Monday pipeline review

```bash
sybill-pp-cli digest --since 7d --agent
```

Every external call from the week, grouped by deal with summary and next steps, as JSON an agent can format.

### Find stalled deals

```bash
sybill-pp-cli deals dark --days 21 --include-uncovered
```

Open deals with no call in 21 days, including open deals that never had a call at all.

### Review pending CRM changes

```bash
sybill-pp-cli crm-autofill --agent --select dealName,field,suggested,current
```

The crmAutofill suggestions Sybill generated, narrowed to just the diff columns.

### Scan transcripts for objections

```bash
sybill-pp-cli patterns --term "pricing|discount|competitor"
```

Count and locate where pricing and competitor talk clusters across cached calls, grouped by deal and stage.

### Narrow a verbose call payload

```bash
sybill-pp-cli conversations get conv_789 --agent --select title,summary.keyTakeaways,summary.nextSteps
```

Conversation detail can be tens of KB; dotted --select pulls only the fields the agent needs.

## Usage

Run `sybill-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Manage accounts

- **`sybill-pp-cli accounts get`** - Get the detailed view for a single account.
- **`sybill-pp-cli accounts list`** - List accounts accessible to the API key's organization.

### conversations

Manage conversations

- **`sybill-pp-cli conversations delete`** - Delete Conversation Ingest
- **`sybill-pp-cli conversations get`** - Get the detailed view for a single conversation.
- **`sybill-pp-cli conversations ingest`** - Ingest Conversation
- **`sybill-pp-cli conversations list`** - List conversations accessible to the API key's organization.

### deals

Manage deals

- **`sybill-pp-cli deals get`** - Get the detailed view for a single deal.
- **`sybill-pp-cli deals list`** - List deals accessible to the API key's organization.

### documents

Manage documents

- **`sybill-pp-cli documents delete`** - Delete Document Ingest
- **`sybill-pp-cli documents get`** - Get Document
- **`sybill-pp-cli documents ingest`** - Ingest Document
- **`sybill-pp-cli documents list`** - List Documents
- **`sybill-pp-cli documents update`** - Update Document Ingest

### health

Manage health

- **`sybill-pp-cli health`** - Health Check

### messages

Manage messages

- **`sybill-pp-cli messages delete`** - Delete Message Ingest
- **`sybill-pp-cli messages get`** - Get Message
- **`sybill-pp-cli messages ingest`** - Ingest Message
- **`sybill-pp-cli messages list`** - List Messages

### object-types

Manage object types

- **`sybill-pp-cli object-types create`** - Create Object Type
- **`sybill-pp-cli object-types delete`** - Delete Object Type
- **`sybill-pp-cli object-types get`** - Get Object Type
- **`sybill-pp-cli object-types list`** - List Object Types
- **`sybill-pp-cli object-types update`** - Update Object Type

### rows

Manage rows

- **`sybill-pp-cli rows delete`** - Delete Row Ingest
- **`sybill-pp-cli rows get`** - Get Row
- **`sybill-pp-cli rows ingest`** - Ingest Row
- **`sybill-pp-cli rows list`** - List Rows
- **`sybill-pp-cli rows update`** - Update Row Ingest Route

### sources

Manage sources

- **`sybill-pp-cli sources create`** - Create Source
- **`sybill-pp-cli sources delete`** - Delete Source
- **`sybill-pp-cli sources get`** - Get Source
- **`sybill-pp-cli sources list`** - List Sources
- **`sybill-pp-cli sources update`** - Update Source


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sybill-pp-cli accounts list

# JSON for scripting and agents
sybill-pp-cli accounts list --json

# Filter to specific fields
sybill-pp-cli accounts list --json --select id,name,status

# Dry run — show the request without sending
sybill-pp-cli accounts list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sybill-pp-cli accounts list --agent
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
sybill-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sybill-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SYBILL_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sybill-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sybill-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SYBILL_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 with {"detail":"Not authenticated"}** — SYBILL_API_KEY is unset or empty; export your sk_live_ key and re-run.
- **403 after authenticating** — Your key is valid but lacks a needed scope (read or ingest). Regenerate the key in the Sybill dashboard with the scope the command needs.
- **429 Rate limit exceeded** — Sybill allows 60 req/min; the client backs off automatically, but lower --limit or sync in smaller windows for large pulls.
- **Recording URL returns 403/expired** — Signed recording URLs expire after 24h; re-fetch the conversation detail to get fresh URLs.
