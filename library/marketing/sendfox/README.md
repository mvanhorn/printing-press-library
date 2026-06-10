# SendFox CLI

**The only SendFox CLI. Write a newsletter with AI, assign a list, send from one command.**

sendfox-pp-cli wraps the full SendFox campaign lifecycle in a terminal-native tool. The write command uses Claude to generate your subject line and email body from a plain-English brief, then creates the draft and sends it. Stats, trends, and contact management are all offline-capable after a sync.

## Install

The recommended path installs both the `sendfox-pp-cli` binary and the `pp-sendfox` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install sendfox
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install sendfox --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

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

Tell your OpenClaw agent (copy this):

```
Install the pp-sendfox skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-sendfox. The skill defines how its required CLI can be installed.
```

## Authentication

Run sendfox-pp-cli setup on first use. It walks you through getting a Personal Access Token at sendfox.com/account/oauth, validates it, fetches your subscriber lists, and saves everything to ~/.config/sendfox/config.yaml. Every subsequent command reads from that config.

## Quick Start

```bash
# First run: enter your token, pick your default list, save config
sendfox-pp-cli setup


# Confirm your subscriber lists loaded correctly
sendfox-pp-cli lists


# AI-write a newsletter draft and assign it to list ID 1
sendfox-pp-cli write --topic 'My best tips for staying consistent' --list 1


# Review the draft before sending
sendfox-pp-cli campaigns list


# Send campaign ID 1 to its assigned list
sendfox-pp-cli campaigns send --id 1


# Check open rates after sending
sendfox-pp-cli campaigns get --id 1

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### AI-powered writing
- **`write`** — Describe a topic in plain English and get a complete, ready-to-send newsletter with subject line, preview text, and full HTML body.

  _Use this when the agent needs to draft and send a newsletter without human copywriting time._

  ```bash
  sendfox-pp-cli write --topic 'How I grew my newsletter to 5000 subscribers' --list 42 --send
  ```
- **`write`** — Get 3 AI-generated subject line options with a rationale for each, then pick one before the draft is created.

  _Use when the agent should let the user choose a subject line before committing to a send._

  ```bash
  sendfox-pp-cli write --topic 'My favorite tools of 2026' --ab-subjects 3
  ```
- **`write`** — Give it a file with one topic per line and it schedules one AI-written newsletter per topic, spaced out over coming weeks.

  _Use when the agent needs to pre-schedule an entire content calendar in one operation._

  ```bash
  sendfox-pp-cli write --from-topics topics.txt --schedule-weekly --list 42
  ```

### Local state that compounds
- **`stats trends`** — See how your open rates, click rates, and unsubscribes have changed over the last N days across all campaigns.

  _Use when the agent needs to assess whether engagement is improving or declining before changing send frequency._

  ```bash
  sendfox-pp-cli stats trends --days 30 --json
  ```
- **`stats funnel`** — A cross-campaign view of sent vs opened vs clicked vs unsubscribed to see your list health at a glance.

  _Use when the agent needs a quick pulse on list engagement before deciding to send or pause._

  ```bash
  sendfox-pp-cli stats funnel --json
  ```
- **`stats best-time`** — Analyzes historical campaign data to identify which days and times correlate with your highest open rates.

  _Use when scheduling a campaign and wanting to optimize the send time based on past performance._

  ```bash
  sendfox-pp-cli stats best-time
  ```

## Usage

Run `sendfox-pp-cli --help` for the full command reference and flag list.

## Commands

### campaigns



- **`sendfox-pp-cli campaigns create`** - Create a new campaign draft
- **`sendfox-pp-cli campaigns delete`** - Delete a draft campaign (only works on unsent drafts)
- **`sendfox-pp-cli campaigns get`** - Get a specific campaign by ID
- **`sendfox-pp-cli campaigns list`** - List all campaigns (100 per page)
- **`sendfox-pp-cli campaigns send`** - Send a draft campaign immediately (must have at least one list assigned)
- **`sendfox-pp-cli campaigns stats`** - Get performance stats for a campaign
- **`sendfox-pp-cli campaigns update`** - Update a draft campaign (only works on unsent drafts)

### contacts



- **`sendfox-pp-cli contacts create`** - Create a new contact
- **`sendfox-pp-cli contacts get`** - Get a specific contact by ID
- **`sendfox-pp-cli contacts list`** - Get all contacts (paginated)
- **`sendfox-pp-cli contacts remove_from_list`** - Remove a contact from a specific list
- **`sendfox-pp-cli contacts unsubscribe`** - Unsubscribe a contact by email

### lists



- **`sendfox-pp-cli lists create`** - Create a new subscriber list
- **`sendfox-pp-cli lists get`** - Get a specific list by ID
- **`sendfox-pp-cli lists list`** - Get all subscriber lists (paginated)

### me



- **`sendfox-pp-cli me get`** - Get authenticated user info


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

## Cookbook

### Write and send a newsletter in one command

```bash
sendfox-pp-cli write --topic 'How I built my audience in 6 months' --list 42 --send
```

Generates subject line, preview text, and full HTML body via Claude, creates the draft in SendFox, assigns it to list 42, and sends it.

### Generate subject line options before committing

```bash
sendfox-pp-cli write --topic 'My favorite tools of 2026' --ab-subjects 3
```

Returns 3 subject line candidates with rationale for each. Pick one before the draft is created.

### Pre-schedule a content calendar from a topics file

```bash
sendfox-pp-cli write --from-topics topics.txt --schedule-weekly --list 42
```

Reads one topic per line from `topics.txt`, generates each campaign, and schedules them 7 days apart starting tomorrow.

### Check engagement trends

```bash
sendfox-pp-cli stats trends --days 30 --json --select open_rate,click_rate,date
```

Returns time-series open and click rate data from local SQLite for agent-readable trend analysis.

### Find the best time to send

```bash
sendfox-pp-cli stats best-time --agent
```

Analyzes past campaign send times against open rates and returns a ranked list of optimal send windows.

### Review list health with a funnel view

```bash
sendfox-pp-cli stats funnel --json
```

Cross-campaign view of sent vs opened vs clicked vs unsubscribed to assess list health before changing send frequency.

### Clone and remix a past campaign

```bash
sendfox-pp-cli campaigns clone --id 18 --subject 'Updated: My best tips for 2026'
```

Copies campaign 18's HTML body and sender details into a new draft with a fresh subject line. Add `--send` to send immediately.

### Sync contacts and search offline

```bash
sendfox-pp-cli sync
sendfox-pp-cli search 'founder' --json --select email,first_name
```

Syncs all contacts and campaigns to local SQLite, then searches offline with no API calls needed.

### Add a contact and assign to a list

```bash
sendfox-pp-cli contacts create --email user@example.com --first-name Jane --lists 42
```

Creates the contact and assigns them to list 42 in one call.

### Export all campaigns for backup

```bash
sendfox-pp-cli export campaigns --format jsonl > campaigns-backup.jsonl
```

Exports all synced campaign data to JSONL for backup or migration.

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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-sendfox -g
```

Then invoke `/pp-sendfox <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add sendfox sendfox-pp-mcp -e SENDFOX_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sendfox-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SENDFOX_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sendfox": {
      "command": "sendfox-pp-mcp",
      "env": {
        "SENDFOX_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
sendfox-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SENDFOX_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sendfox-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SENDFOX_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Error: SENDFOX_TOKEN not set or config not found** — Run sendfox-pp-cli setup to create your config file
- **Error 400: No lists assigned** — Assign a list first: sendfox-pp-cli campaigns update --id {id} --lists 42
- **Error 409: Campaign already sent** — Use sendfox-pp-cli campaigns clone --id {id} to remix a sent campaign into a new draft
- **Error 422: subject cannot start with RE: or FWD:** — Edit the subject line — SendFox rejects subjects beginning with RE: or FWD:
- **Rate limit hit (429)** — The CLI respects X-RateLimit-Remaining headers and backs off automatically

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**robinsloan/mailchimp-cli**](https://github.com/robinsloan/mailchimp-cli) — Ruby (312 stars)
- [**damientilman/mailchimp-mcp-server**](https://github.com/damientilman/mailchimp-mcp-server) — JavaScript (89 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
