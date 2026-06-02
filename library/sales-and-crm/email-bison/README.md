# Email Bison CLI

**Every Email Bison campaign, lead, and reply on the command line, plus a local database and cross-entity analytics no other Email Bison tool has.**

A Go CLI for the Email Bison cold-email API with a local SQLite store, cursor-based sync, and offline FTS. Beyond mirroring every endpoint, it answers questions a single API call cannot: which campaigns are under their daily cap (campaigns headroom), which senders are degraded but still live (sender-emails health), which leads are stuck mid-sequence (leads stale), and which A/B variant is winning (campaigns variants).

## Install

The recommended path installs both the `email-bison-pp-cli` binary and the `pp-email-bison` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install email-bison
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install email-bison --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install email-bison --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install email-bison --agent claude-code
npx -y @mvanhorn/printing-press-library install email-bison --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/email-bison-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-email-bison --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-email-bison --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-email-bison skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-email-bison. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/email-bison-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EMAIL_BISON_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "email-bison": {
      "command": "email-bison-pp-mcp",
      "env": {
        "EMAIL_BISON_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Email Bison is self-hosted: each workspace has its own dedicated instance domain. Set EMAIL_BISON_BASE_URL to your instance (default https://dedi.emailbison.com) and EMAIL_BISON_API_KEY to a workspace api-user token from Settings -> Developer API. Every token is scoped to one workspace.

## Quick Start

```bash
# confirm the base URL and token resolve and the API is reachable
email-bison-pp-cli doctor

# pull campaigns, leads, senders, and replies into the local store
email-bison-pp-cli sync

# see which campaigns are sending below their daily cap
email-bison-pp-cli campaigns headroom --agent

# roll up interested replies from the last day
email-bison-pp-cli replies interested --since 24h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local joins that compound
- **`campaigns headroom`** — See which launched campaigns are sending below their daily cap, at cap, or idle, in one table.

  _Reach for this when an agent needs to find under-delivering campaigns across an account without opening every campaign in the UI._

  ```bash
  email-bison-pp-cli campaigns headroom --agent
  ```
- **`sender-emails health`** — One board joining sender state, attached campaigns, and recent bounces to spot dead or over-assigned inboxes.

  _Use before a deliverability audit to find senders that are degraded but still actively sending._

  ```bash
  email-bison-pp-cli sender-emails health --agent
  ```
- **`leads stale`** — Find leads stuck mid-sequence: in a live campaign, already emailed, no reply, and no scheduled next send.

  _Use weekly to surface leads silently rotting in a sequence so they can be re-engaged or cleaned out._

  ```bash
  email-bison-pp-cli leads stale --days 7 --agent
  ```

### Reply triage
- **`replies interested`** — Every reply marked interested across all campaigns since a timestamp, joined to its lead, campaign, and sender.

  _Reach for this for a daily 'what got interested since yesterday' triage across the whole account._

  ```bash
  email-bison-pp-cli replies interested --since 24h --agent
  ```
- **`replies triage`** — An oldest-first worklist of pending replies with lead and campaign context, ready to pipe into mark-interested or follow-up push.

  _Use as the entry point for a morning master-inbox triage loop an agent can work top to bottom._

  ```bash
  email-bison-pp-cli replies triage --agent
  ```

### Campaign optimization
- **`campaigns variants`** — Per A/B sequence-step variant, the reply rate and interested rate computed from local data.

  _Reach for this when deciding which subject/body variant to keep in a multi-step campaign._

  ```bash
  email-bison-pp-cli campaigns variants 6 --agent
  ```
- **`campaigns preflight`** — Before resuming, check locally that a campaign has a schedule, at least one step, a sender, leads, and that every merge tag exists as a custom variable.

  _Run this before launch to catch broken {VARIABLE} renders and missing senders that would silently kill a send._

  ```bash
  email-bison-pp-cli campaigns preflight 6 --agent
  ```

## Recipes


### Morning inbox triage

```bash
email-bison-pp-cli replies triage --agent
```

Pull an oldest-first queue of pending replies, then mark-interested or push the good ones into a follow-up campaign.

### Pre-launch safety check

```bash
email-bison-pp-cli campaigns preflight 6 --agent
```

Confirm a campaign has a schedule, sequence, senders, leads, and valid merge tags before calling resume.

### Deliverability sweep

```bash
email-bison-pp-cli sender-emails health --agent --select email,state,live_campaigns,recent_bounces
```

Surface degraded senders still attached to live campaigns, narrowing a verbose response to the fields that matter.

### Find rotting leads

```bash
email-bison-pp-cli leads stale --days 7 --agent
```

List leads emailed a week ago with no reply and no next send so they can be re-engaged or removed.

### Launch a campaign

```bash
email-bison-pp-cli campaigns resume campaign 6 --dry-run
```

Preview the launch call, then drop --dry-run to start sending once preflight is clean.

## Usage

Run `email-bison-pp-cli --help` for the full command reference and flag list.

## Commands

### campaigns

Manage campaigns

- **`email-bison-pp-cli campaigns create`** - Create a campaign. Returns the new campaign ID.
- **`email-bison-pp-cli campaigns delete-sequence-step`** - Delete a sequence step.
- **`email-bison-pp-cli campaigns get`** - Get a campaign and its settings.
- **`email-bison-pp-cli campaigns list`** - List campaigns in the workspace.
- **`email-bison-pp-cli campaigns list-schedule-templates`** - List saved schedule templates.
- **`email-bison-pp-cli campaigns send-test-sequence-step`** - Send a test email for a sequence step.

### custom-variables

Manage custom variables

- **`email-bison-pp-cli custom-variables create`** - Create a custom variable.
- **`email-bison-pp-cli custom-variables list`** - List custom variables in the workspace.

### leads

Manage leads

- **`email-bison-pp-cli leads create`** - Create a single lead.
- **`email-bison-pp-cli leads list`** - List leads (contacts) in the workspace.
- **`email-bison-pp-cli leads update`** - Update a lead and its custom variables.

### replies

Manage replies

- **`email-bison-pp-cli replies`** - List replies in the master inbox.

### scheduled-emails

Manage scheduled emails

- **`email-bison-pp-cli scheduled-emails <lead_id_or_email>`** - Get scheduled emails for a lead.

### sender-emails

Manage sender emails

- **`email-bison-pp-cli sender-emails list`** - List connected sender email accounts.
- **`email-bison-pp-cli sender-emails update`** - Update a sender email account's settings.

### tags

Manage tags

- **`email-bison-pp-cli tags attach-to-campaigns`** - Attach tags to campaigns.
- **`email-bison-pp-cli tags attach-to-leads`** - Attach tags to leads.
- **`email-bison-pp-cli tags attach-to-sender-emails`** - Attach tags to sender emails.
- **`email-bison-pp-cli tags create`** - Create a custom tag.
- **`email-bison-pp-cli tags delete`** - Delete a custom tag.
- **`email-bison-pp-cli tags list`** - List custom tags.
- **`email-bison-pp-cli tags remove-from-campaigns`** - Remove tags from campaigns.
- **`email-bison-pp-cli tags remove-from-leads`** - Remove tags from leads.
- **`email-bison-pp-cli tags remove-from-sender-emails`** - Remove tags from sender emails.

### users

Manage users

- **`email-bison-pp-cli users`** - List users in the current workspace (also validates the token).

### webhook-events

Manage webhook events

- **`email-bison-pp-cli webhook-events`** - Send a test webhook event.

### webhooks

Manage webhooks

- **`email-bison-pp-cli webhooks`** - Create a webhook subscription.

### workspaces

Manage workspaces

- **`email-bison-pp-cli workspaces create-token`** - Create an api-user token for a workspace (requires a super-admin token).
- **`email-bison-pp-cli workspaces list`** - List workspaces (requires a super-admin token).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
email-bison-pp-cli campaigns list

# JSON for scripting and agents
email-bison-pp-cli campaigns list --json

# Filter to specific fields
email-bison-pp-cli campaigns list --json --select id,name,status

# Dry run — show the request without sending
email-bison-pp-cli campaigns list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
email-bison-pp-cli campaigns list --agent
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
email-bison-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/email-bison-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EMAIL_BISON_API_KEY` | per_call | Yes | Workspace api-user token from Settings -> Developer API. |
| `EMAIL_BISON_BASE_URL` | per_call | No | Your dedicated instance URL. Defaults to `https://dedi.emailbison.com`; override for self-hosted workspaces. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `email-bison-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `email-bison-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EMAIL_BISON_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — Set EMAIL_BISON_API_KEY to a valid api-user token and confirm EMAIL_BISON_BASE_URL points at your dedicated instance, not dedi.emailbison.com.
- **Commands hit the wrong workspace** — Each api-user token is scoped to one workspace; use the token minted inside the target workspace.
- **HTTP 429 during a large sync** — The API allows about 10 requests/second; the client backs off on the retry_after header, so let the sync finish rather than re-running it.
- **Merge tags render empty in a campaign** — Run campaigns preflight <id>; every {VARIABLE} in a sequence body must exist as a custom variable in the workspace.
