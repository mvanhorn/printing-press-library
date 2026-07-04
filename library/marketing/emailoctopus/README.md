# EmailOctopus CLI

**Every EmailOctopus v2 endpoint, plus the cross-list joins, churn diffs, and rate-budgeted bulk operations the API can't do on its own.**

EmailOctopus is the only major email-marketing platform whose free tier includes API access — but until now no maintained v2 CLI or SDK existed. This CLI ships all 25 endpoints with --json, --select, --csv, --dry-run, plus a local SQLite store that powers cross-list dedupe, per-contact engagement scoring, list churn diffs over time, and rate-budgeted bulk delete.

Created by [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `emailoctopus-pp-cli` binary and the `pp-emailoctopus` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install emailoctopus
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install emailoctopus --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install emailoctopus --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install emailoctopus --agent claude-code
npx -y @mvanhorn/printing-press-library install emailoctopus --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/emailoctopus/cmd/emailoctopus-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/emailoctopus-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install emailoctopus --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-emailoctopus --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-emailoctopus --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install emailoctopus --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/emailoctopus-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `EMAILOCTOPUS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "emailoctopus": {
      "command": "emailoctopus-pp-mcp",
      "env": {
        "EMAILOCTOPUS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Generate a v2 key at https://api.emailoctopus.com/developer/api-keys/create and export it as `EMAILOCTOPUS_API_KEY`. Keys created before October 2024 are v1-only and will return 401 against the v2 API — regenerate if you see auth errors.

## Quick Start

```bash
# Verify auth and reachability before doing anything else.
emailoctopus-pp-cli doctor

# Inventory your lists and grab the list_id you'll use below.
emailoctopus-pp-cli lists list --json

# Pull every list, contact, tag, field, and campaign into the local SQLite store.
emailoctopus-pp-cli sync --full

# Surface cold subscribers — the API has no equivalent.
emailoctopus-pp-cli contacts engagement --list <list_id> --inactive-since 90d --json

# Paste-ready campaign report in one command.
emailoctopus-pp-cli campaigns digest 071f24b2-51cd-11f1-a3ce-11fd783017da --md

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local joins the API can't do
- **`contacts engagement`** — Score every contact by opens, clicks, and inactive-since across all campaigns. The API has no engagement-history endpoint; we synthesize it locally.

  _Pick this when an agent needs to find cold subscribers, build a reactivation cohort, or score engagement without paginating every campaign report by hand._

  ```bash
  emailoctopus-pp-cli contacts engagement --list <list_id> --inactive-since 90d --json
  ```
- **`contacts dedupe`** — Find contacts that appear on multiple lists in the account, optionally consolidating them onto one canonical list.

  _Pick this when an agent needs to clean up subscriber duplication, audit list sprawl, or plan a merge across multiple lists._

  ```bash
  emailoctopus-pp-cli contacts dedupe --json
  ```
- **`tags intersect`** — Find contacts matching boolean combinations of tags: --has trial-started --not activated returns the trial cohort that hasn't converted.

  _Pick this when an agent needs to segment subscribers by tag combinations for targeted outreach, churn reactivation, or audience reporting._

  ```bash
  emailoctopus-pp-cli tags intersect --list <list_id> --has trial-started --not activated --json
  ```

### Workflow accelerators
- **`campaigns digest`** — One-shot campaign report combining summary, top-N links, contact-level breakdown, and per-domain opens — rendered for terminal or Markdown paste.

  _Pick this when an agent needs to summarize a campaign's results for a stakeholder doc without screen-scraping the EmailOctopus dashboard._

  ```bash
  emailoctopus-pp-cli campaigns digest <campaign_id> --md
  ```
- **`contacts sync-csv`** — Push a CSV into EmailOctopus with mapped fields and tags. Dry-runs the diff against the local store first, then chunks into batch-upsert calls paced under the rate limit.

  _Pick this when an agent needs to atomically sync a CSV of contacts with tag/field mapping and pre-flight the change before applying it._

  ```bash
  emailoctopus-pp-cli contacts sync-csv ./subscribers.csv --list <list_id> --map email=Email,tag.plan=Plan --dry-run
  ```

### Local snapshots over time
- **`lists diff`** — Show contacts touched in this list since a relative time. Surfaces the change-set the API can't return — useful for incremental syncs, audit logs, or alerting on recent activity.

  _Pick this when an agent needs to see which contacts were touched in the last hour/day/week or run an incremental change-detection workflow._

  ```bash
  emailoctopus-pp-cli lists diff <list_id> --since yesterday --json
  ```

### Mutation safety
- **`contacts bulk-delete`** — Delete many contacts matching a local predicate, paced under the 10/sec API limit with a resumable progress file.

  _Pick this when an agent needs to clean up unsubscribed, bounced, or stale contacts in bulk without hitting 429s or losing progress mid-run._

  ```bash
  emailoctopus-pp-cli contacts bulk-delete --list <list_id> --where 'status=unsubscribed' --rate 8 --dry-run
  ```
- **`automations trigger-batch`** — Queue an automation for many contacts from stdin or CSV, paced under the rate limit with retry on 429.

  _Pick this when an agent needs to start an automation for a batch of contacts (trial-ending cohort, plan-upgrade celebration) without writing loop+backoff boilerplate._

  ```bash
  cat trial-ending.csv | emailoctopus-pp-cli automations trigger-batch <automation_id> --stdin
  ```

## Usage

Run `emailoctopus-pp-cli --help` for the full command reference and flag list.

## Commands

### automations

An automation is a sequence of automated steps triggered by an event, such as when a contact subscribes to a list or is tagged.
Automations allow you to automatically send emails, update fields, apply tags and more.

### campaigns

A campaign is generally used to send a one-off, timely email to some or all of your subscribers. For example you may use a campaign to send the latest edition of your weekly newsletter, or to announce a new feature in your product.

- **`emailoctopus-pp-cli campaigns get`** - Get all campaigns
- **`emailoctopus-pp-cli campaigns id-get`** - Get campaign

### lists

A list is a collection of contacts. Every one of your contacts will exist inside a list. The majority of our users only require one list, but multiple lists can be created and configured with different fields and tags in order to organise distinct groups of contacts.

- **`emailoctopus-pp-cli lists get`** - Get all lists
- **`emailoctopus-pp-cli lists id-delete`** - Delete a list
- **`emailoctopus-pp-cli lists id-get`** - Get list
- **`emailoctopus-pp-cli lists id-put`** - Update list
- **`emailoctopus-pp-cli lists post`** - Create list

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
emailoctopus-pp-cli campaigns get

# JSON for scripting and agents
emailoctopus-pp-cli campaigns get --json

# Filter to specific fields
emailoctopus-pp-cli campaigns get --json --select id,name,status

# Dry run — show the request without sending
emailoctopus-pp-cli campaigns get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
emailoctopus-pp-cli campaigns get --agent
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
emailoctopus-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/emailoctopus-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `EMAILOCTOPUS_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `emailoctopus-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $EMAILOCTOPUS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any v2 endpoint** — Your API key was minted before October 2024. Regenerate it at https://api.emailoctopus.com/developer/api-keys/create and re-export EMAILOCTOPUS_API_KEY.
- **429 rate-limited during a bulk loop** — Use `contacts bulk-delete --rate 8` or `automations trigger-batch` — both pace under the documented 10 req/sec budget and respect the X-RateLimit-Retry-After header.
- **`contacts engagement` returns empty results** — Run `emailoctopus-pp-cli sync --full` first. Engagement scoring joins local campaign-report tables that only populate after sync.
- **`contacts sync-csv` shows the wrong mapping** — Always run with `--dry-run` first; the dry-run diff against the local store shows exactly which contacts will be created, updated, or no-op'd before any API call fires.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Activepieces EmailOctopus piece**](https://github.com/activepieces/activepieces) — TypeScript (18000 stars)
- [**kartoffelkraft/email-octopus-ts**](https://github.com/kartoffelkraft/email-octopus-ts) — TypeScript (14 stars)
- [**tubbo/email_octopus**](https://github.com/tubbo/email_octopus) — Ruby (8 stars)
- [**goran-popovic/email-octopus-php**](https://github.com/goran-popovic/email-octopus-php) — PHP (5 stars)
- [**wthomsen/email-octopus**](https://github.com/wthomsen/email-octopus) — JavaScript (3 stars)
- [**Zapier MCP for EmailOctopus**](https://zapier.com/mcp/emailoctopus) — hosted

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
