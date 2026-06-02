# Zapmail CLI

**Every Zapmail dashboard action, plus an offline SQLite mirror of your whole mailbox and domain fleet, with deliverability rollups and renewal forecasting no other Zapmail tool ships.**

Drive Zapmail from the command line with agent-native output (--json, --select, typed exit codes) and a --dry-run guard on everything that spends money. Sync your domains, mailboxes, subscriptions, and exports into local SQLite, then run fleet queries the dashboard cannot: fleet health rollups, warmed mailbox finders, failed-mailbox triage, renewal forecasts, and cost-per-active-mailbox. (v1 operates in your primary workspace.)

## Install

The recommended path installs both the `zapmail-pp-cli` binary and the `pp-zapmail` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install zapmail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install zapmail --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install zapmail --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install zapmail --agent claude-code
npx -y @mvanhorn/printing-press-library install zapmail --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zapmail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-zapmail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-zapmail --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-zapmail skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-zapmail. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/zapmail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ZAPMAIL_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "zapmail": {
      "command": "zapmail-pp-mcp",
      "env": {
        "ZAPMAIL_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Zapmail uses a single API key sent in the x-auth-zapmail header (raw token, not Bearer). Get it at Dashboard > Settings > Integrations > API and set ZAPMAIL_API_KEY.

## Quick Start

```bash
# Confirm the API key works and the API is reachable
zapmail-pp-cli doctor

# See your plan, mailbox counts, and wallet balance
zapmail-pp-cli user get

# Mirror workspaces, domains, mailboxes, subscriptions, and exports into local SQLite
zapmail-pp-cli sync

# List warmed mailboxes plus paid-but-unprovisioned capacity
zapmail-pp-cli mailboxes idle --json

# Roll up unhealthy and abused domains across your synced fleet
zapmail-pp-cli analytics --type fleet-health --group-by workspace

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Fleet intelligence
- **`analytics --type fleet-health --group-by workspace`** — See every unhealthy or abused domain in your synced fleet in one table, instead of checking domains one at a time in the dashboard.

  _Reach for this first thing to triage deliverability across your whole fleet in one command._

  ```bash
  zapmail-pp-cli analytics --type fleet-health --group-by workspace --json
  ```
- **`mailboxes idle`** — List warmed mailboxes ready to send and report how many paid mailboxes are not yet provisioned, so you recover capacity you are paying for.

  _Use this to recover billable send capacity you forgot to put to work before a client notices low volume._

  ```bash
  zapmail-pp-cli mailboxes idle --json
  ```
- **`mailboxes failed`** — Surface every mailbox that failed or stalled during creation, grouped by domain and workspace, ready to retry.

  _Catch silently-failed provisioning before it shows up as missing send volume mid-campaign._

  ```bash
  zapmail-pp-cli mailboxes failed --json
  ```

### Cost and capacity
- **`analytics --type renewals --group-by week`** — Forecast which subscriptions and domains renew in the coming weeks and what each renewal will cost, so nothing lapses unexpectedly.

  _Use this on a Friday capacity review so a missed renewal never silently drops a sending domain._

  ```bash
  zapmail-pp-cli analytics --type renewals --group-by week --json
  ```
- **`analytics --type cost-efficiency --group-by workspace`** — Divide active subscription spend by the number of assigned mailboxes to expose where money is being wasted.

  _Reach for this to answer 'what are we actually paying per working mailbox' before a budget review._

  ```bash
  zapmail-pp-cli analytics --type cost-efficiency --group-by workspace --json
  ```
- **`analytics --type capacity --group-by workspace`** — Show purchased vs assigned vs available mailbox counts so you see unused capacity at a glance.

  _Use this to decide whether to assign more mailboxes or stop paying for capacity you aren't using._

  ```bash
  zapmail-pp-cli analytics --type capacity --group-by workspace --json
  ```

### Agent-native plumbing
- **`exports watch`** — Poll one export to completion and exit non-zero if it fails, so you can gate a script on a sequencer export finishing.

  _Use this in a script after starting an export so the next step only runs once the mailboxes actually land in the sequencer._

  ```bash
  zapmail-pp-cli exports watch --export-id 12345
  ```

## Recipes


### Morning deliverability triage

```bash
zapmail-pp-cli sync && zapmail-pp-cli analytics --type fleet-health --group-by workspace --agent --select workspace,unhealthyCount,abusedCount
```

Sync the fleet, then narrow the rollup to just the at-risk counts per workspace for a fast scan.

### Recover wasted send capacity

```bash
zapmail-pp-cli mailboxes idle --json | jq '.[].email'
```

List warmed-but-unassigned mailboxes so you can put paid-for inboxes back to work.

### Clean export into a sequencer

```bash
zapmail-pp-cli exports mailboxes --apps SMARTLEAD --contains client.com --dry-run
```

Preview exactly which mailboxes would be exported before sending them, no real export performed.

### Forecast next week's renewals

```bash
zapmail-pp-cli analytics --type renewals --group-by week --csv
```

Get a week-bucketed renewal cost forecast as CSV for a spreadsheet or budget review.

### Send a one-off from a connected inbox

```bash
zapmail-pp-cli inbox send --account you@example.com --to prospect@example.com --subject 'Quick q' --body 'Hi there' --dry-run
```

Compose and preview an outbound send from a Zapbox-connected account before actually sending.

## Usage

Run `zapmail-pp-cli --help` for the full command reference and flag list.

## Commands

### dns

DNS records on assigned domains

- **`zapmail-pp-cli dns add`** - Add one or more DNS records to an assigned domain
- **`zapmail-pp-cli dns list`** - Get all DNS records for an assigned domain

### domains

Domains connected to or purchased through Zapmail

- **`zapmail-pp-cli domains ai-finder`** - Generate available domain name suggestions from keywords using AI
- **`zapmail-pp-cli domains assignable`** - List domains that have free capacity to assign new mailboxes
- **`zapmail-pp-cli domains available-bulk`** - Check registration availability for multiple domain names at once
- **`zapmail-pp-cli domains health-score`** - Retrieve the deliverability health score and abuse flag for one domain
- **`zapmail-pp-cli domains list`** - Retrieve all domains with status, DNS, forwarding, and assigned-mailbox counts

### exports

Export mailboxes to third-party sequencers

- **`zapmail-pp-cli exports accounts`** - List connected third-party export accounts for an app
- **`zapmail-pp-cli exports add-account`** - Add third-party export account credentials
- **`zapmail-pp-cli exports mailboxes`** - Export mailboxes to one or more sequencer apps (Smartlead, Instantly, ReachInbox, etc.)
- **`zapmail-pp-cli exports status`** - Check the status of a running or completed export

### inbox

Zapbox - read and send email from connected mailboxes

- **`zapmail-pp-cli inbox accounts`** - List inbox-connected accounts available in Zapbox
- **`zapmail-pp-cli inbox emails`** - Fetch recent emails for a connected account
- **`zapmail-pp-cli inbox search`** - Search emails across connected accounts
- **`zapmail-pp-cli inbox send`** - Send an email from a connected account (real outbound send)

### mailboxes

Mailboxes provisioned on Zapmail domains

- **`zapmail-pp-cli mailboxes get`** - Get details of a single mailbox by its ID
- **`zapmail-pp-cli mailboxes list`** - Retrieve all mailboxes, grouped by domain, with counts and warmup status

### subscriptions

Mailbox subscriptions and plans

- **`zapmail-pp-cli subscriptions`** - Get all subscriptions with plan, price, mailbox quantity, and billing period

### user

Authenticated account details

- **`zapmail-pp-cli user`** - Retrieve the authenticated user: plan, mailbox counts, wallet balance

### wallet

Account wallet balance and auto-recharge

- **`zapmail-pp-cli wallet`** - Get current wallet balance (USD) and auto-recharge settings

### workspaces

Workspaces (isolated domain + mailbox containers)

- **`zapmail-pp-cli workspaces create`** - Create a new workspace, optionally with billing details
- **`zapmail-pp-cli workspaces list`** - Retrieve all workspaces for the authenticated account


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
zapmail-pp-cli dns list --id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
zapmail-pp-cli dns list --id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
zapmail-pp-cli dns list --id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
zapmail-pp-cli dns list --id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
zapmail-pp-cli dns list --id 550e8400-e29b-41d4-a716-446655440000 --agent
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
zapmail-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/zapmail-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ZAPMAIL_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `zapmail-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `zapmail-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ZAPMAIL_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 / 403 on every call** — Set ZAPMAIL_API_KEY to the token from Dashboard > Settings > Integrations > API, then run 'zapmail-pp-cli doctor'.
- **429 Too Many Requests** — Zapmail limits general calls to 5/sec and 20/min; let sync back off, or reduce --limit on list commands.
- **Fleet analytics look empty or stale** — Run 'zapmail-pp-cli sync' first; analytics read the local mirror, not the live API.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**zapmail-mcp**](https://github.com/dsouzaalan/zapmail-mcp) — TypeScript
- [**coldoutboundskills**](https://github.com/growthenginenowoslawski/coldoutboundskills) — Markdown

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
