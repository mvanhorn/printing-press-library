# SmartLead CLI

**Every SmartLead API feature, plus a local mirror that answers campaign-health, silent-lead, and cross-campaign dedupe questions the API cannot.**

Beyond wrapping all 41 SmartLead REST endpoints with JSON-first, agent-native output, this CLI adds a local SQLite mirror no other SmartLead tool has. Sync once, then run health, silent, dupes, sender-health, warmup-gate, and drift offline — cross-campaign questions that otherwise cost a multi-call API loop per answer.

Created by [@bossriceshark](https://github.com/bossriceshark) (bossriceshark).

## Install

The recommended path installs both the `smartlead-pp-cli` binary and the `pp-smartlead` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install smartlead
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install smartlead --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install smartlead --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install smartlead --agent claude-code
npx -y @mvanhorn/printing-press-library install smartlead --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/smartlead/cmd/smartlead-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/smartlead-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-smartlead --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-smartlead --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install smartlead --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/smartlead-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SMARTLEAD_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "smartlead": {
      "command": "smartlead-pp-mcp",
      "env": {
        "SMARTLEAD_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

SmartLead authenticates with an API key passed as the api_key query parameter. Set SMARTLEAD_API_KEY in your environment (find the key under Settings -> API in the SmartLead app). No OAuth, no login flow.

## Quick Start

```bash
# Confirm the API key is set and SmartLead is reachable before anything else.
smartlead-pp-cli doctor

# Mirror campaigns, leads, email accounts, and statistics into the local SQLite store.
smartlead-pp-cli sync

# List every campaign now served instantly from the local mirror.
smartlead-pp-cli campaigns list --json

# Get the bounce/reply/silent-lead scorecard across all campaigns at once.
smartlead-pp-cli health --json

# Check whether a domain is already pitched before adding new leads.
smartlead-pp-cli dupes --domain example.com --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`health`** — One-shot scorecard for every campaign — bounce rate, reply rate, silent-lead count, sender count, and a stale flag — without clicking through the dashboard.

  _Reach for this first when an agent needs to know which campaigns are healthy before drilling into any one of them._

  ```bash
  smartlead-pp-cli health --json
  ```
- **`silent`** — Finds leads that were emailed but have not replied within N days — the exact set to follow up with or retire.

  _Use this to build a follow-up list instead of paging the whole lead set and diffing timestamps by hand._

  ```bash
  smartlead-pp-cli silent --campaign 12345 --days 7 --json
  ```
- **`dupes`** — Scans the whole lead mirror for emails or domains that appear in two or more campaigns; --domain prints the full pitch ledger for one site.

  _Run this before adding leads to a campaign to avoid double-contacting a prospect already in flight._

  ```bash
  smartlead-pp-cli dupes --domain example.com --json
  ```
- **`drift`** — Computes week-over-week reply, open, and bounce deltas for a campaign by querying analytics one seven-day window at a time.

  _Use this to catch a campaign decaying over time rather than judging it from a single point-in-time stat._

  ```bash
  smartlead-pp-cli drift --campaign 12345 --weeks 4 --json
  ```

### Deliverability intelligence
- **`sender-health`** — Ranks every email sender account by a composite of inbox-warmup landing rate, SMTP/IMAP connection health, and sending utilization.

  _Reach for this to find which sender accounts are dragging deliverability before they tank a campaign._

  ```bash
  smartlead-pp-cli sender-health --json
  ```
- **`warmup-gate`** — Checks each sender account against warmup thresholds (--min-days, --min-inbox-rate); with --strict it exits non-zero when any account fails — a scriptable launch gate.

  _Call this in a launch script to block attaching a sender account that is not warmed up yet._

  ```bash
  smartlead-pp-cli warmup-gate --account 6789 --json
  ```

## Usage

Run `smartlead-pp-cli --help` for the full command reference and flag list.

## Commands

### Campaigns

| Command | Description |
| --- | --- |
| `campaigns list` | List every email campaign for the account |
| `campaigns get <campaign_id>` | Fetch one campaign by ID |
| `campaigns create` | Create a new campaign |
| `campaigns delete <campaign_id>` | Permanently delete a campaign |
| `campaigns status update-campaign <campaign_id>` | Start, pause, or stop a campaign |
| `campaigns schedule update-campaign <campaign_id>` | Update a campaign's sending schedule |
| `campaigns settings update-campaign <campaign_id>` | Update tracking and stop-condition settings |
| `campaigns sequences get <campaign_id>` | Get the email sequence steps |
| `campaigns sequences save <campaign_id>` | Save the email sequence steps |
| `campaigns analytics get-campaign <campaign_id>` | Top-level analytics for a campaign |
| `campaigns analytics-by-date get-campaign <campaign_id>` | Analytics for a date range |
| `campaigns statistics get-campaign <campaign_id>` | Per-lead send statistics |
| `campaigns webhooks list-campaign <campaign_id>` | List webhooks on a campaign |
| `campaigns webhooks upsert-campaign <campaign_id>` | Create or update a campaign webhook |
| `campaigns webhooks delete-campaign <campaign_id>` | Delete a campaign webhook |

### Campaign leads

| Command | Description |
| --- | --- |
| `campaigns leads list-campaign <campaign_id>` | List leads in a campaign |
| `campaigns leads add-to-campaign <campaign_id>` | Add leads to a campaign |
| `campaigns leads update-in-campaign <campaign_id>` | Update a lead's fields within a campaign |
| `campaigns leads update-category <campaign_id>` | Set a lead's category in a campaign |
| `campaigns leads pause-in-campaign <campaign_id>` | Pause sends to one lead (reversible) |
| `campaigns leads resume-in-campaign <campaign_id>` | Resume a paused lead |
| `campaigns leads unsubscribe-from-campaign <campaign_id>` | Unsubscribe a lead from one campaign |
| `campaigns leads delete-from-campaign <campaign_id>` | Remove a lead from a campaign |
| `campaigns leads get-message-history <campaign_id>` | Full email thread for a lead |
| `campaigns leads-export export-campaign-leads <campaign_id>` | Export campaign leads as CSV |
| `campaigns email-accounts get-campaign <campaign_id>` | List sender accounts on a campaign |
| `campaigns email-accounts add-campaign <campaign_id>` | Attach sender accounts to a campaign |
| `campaigns email-accounts remove-campaign <campaign_id>` | Detach sender accounts from a campaign |
| `campaigns reply-email-thread reply-to-lead-thread` | Reply within an existing campaign thread |

### Leads

| Command | Description |
| --- | --- |
| `leads get-by-email` | Look up a lead by email address |
| `leads update <lead_id>` | Update a lead's account-level fields |
| `leads list-categories` | List all lead categories for the account |
| `leads campaigns get-lead <lead_id>` | List every campaign a lead belongs to |
| `leads unsubscribe lead-from-all <lead_id>` | Unsubscribe a lead from every campaign |
| `leads add-domain-block-list` | Add domains to the account-wide block list |

### Email accounts

| Command | Description |
| --- | --- |
| `email-accounts list` | List all email sender accounts |
| `email-accounts create` | Add a new email sender account |
| `email-accounts update <id>` | Update an email sender account |
| `email-accounts reconnect-failed` | Reconnect all disconnected accounts |
| `email-accounts warmup set-settings <id>` | Configure inbox warmup for an account |
| `email-accounts warmup-stats get <id>` | Warmup performance stats for an account |

### Clients

| Command | Description |
| --- | --- |
| `client list` | List all whitelabel clients |
| `client create` | Create a whitelabel client |

### Local mirror & insight

| Command | Description |
| --- | --- |
| `sync` | Sync API data into the local SQLite mirror |
| `search <query>` | Full-text search across synced data or live API |
| `analytics` | Group-by analytics over locally synced data |
| `health` | Campaign health scorecard from the mirror |
| `silent` | Leads emailed but silent for N days |
| `dupes` | Leads or domains contacted across multiple campaigns |
| `drift` | Week-over-week open/reply/bounce drift for a campaign |
| `sender-health` | Rank sender accounts by a deliverability composite |
| `warmup-gate` | Pass/fail launch gate for sender warmup readiness |
| `email-campaigns` | Reply via the master inbox using only a stats ID |

### Utilities

| Command | Description |
| --- | --- |
| `doctor` | Check CLI health, credentials, and connectivity |
| `auth` | Manage authentication for SmartLead |
| `export` / `import` | Back up or restore data as JSONL |
| `tail` | Stream live changes by polling the API |
| `workflow` | Compound workflows that combine multiple API calls |
| `api` | Browse all API endpoints by interface name |
| `which` | Find the command that implements a capability |
| `profile` | Named sets of flags saved for reuse |
| `agent-context` | Emit structured JSON describing this CLI for agents |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
smartlead-pp-cli campaigns list

# JSON for scripting and agents
smartlead-pp-cli campaigns list --json

# Filter to specific fields
smartlead-pp-cli campaigns list --json --select id,name,status

# Dry run — show the request without sending
smartlead-pp-cli campaigns list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
smartlead-pp-cli campaigns list --agent
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

## Cookbook

```bash
# 1. Confirm the API key works before doing anything else
smartlead-pp-cli doctor

# 2. Mirror the whole account into local SQLite, then work offline
smartlead-pp-cli sync
smartlead-pp-cli sync --since 24h          # incremental refresh
smartlead-pp-cli sync --resources campaigns,email-accounts

# 3. Triage every campaign at once — bounce/reply/silent scorecard
smartlead-pp-cli health --json

# 4. Build a follow-up list: leads emailed but silent 14+ days
smartlead-pp-cli silent --days 14 --json
smartlead-pp-cli silent --campaign 12345 --days 7 --json

# 5. Catch a decaying campaign before it flatlines
smartlead-pp-cli drift --campaign 12345 --weeks 6 --json

# 6. Avoid double-pitching a prospect already in flight
smartlead-pp-cli dupes --domain example.com --json
smartlead-pp-cli dupes --email "$PROSPECT_EMAIL" --json

# 7. Rank sender accounts by deliverability composite
smartlead-pp-cli sender-health --limit 10 --json

# 8. Block a launch when any sender is not warmed up (scriptable gate)
smartlead-pp-cli warmup-gate --min-days 7 --min-inbox-rate 0.9 --strict

# 9. Pause a campaign, then resume it later
smartlead-pp-cli campaigns status update-campaign 12345 --status PAUSED
smartlead-pp-cli campaigns status update-campaign 12345 --status START

# 10. Add leads to a campaign from a JSON file
cat leads.json | smartlead-pp-cli campaigns leads add-to-campaign 12345 --stdin

# 11. Group-by analytics over the local mirror
smartlead-pp-cli analytics --type campaigns --group-by status --json

# 12. Full-text search across everything synced
smartlead-pp-cli search "acme corp" --type leads --limit 25 --json

# 13. Inspect a campaign's email sequence steps
smartlead-pp-cli campaigns sequences get 12345 --json

# 14. Reply to a lead via the master inbox using only a stats ID
smartlead-pp-cli email-campaigns --email-stats-id abc123 --email-body "Thanks!"
```

## Health Check

```bash
smartlead-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/smartlead-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SMARTLEAD_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `smartlead-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SMARTLEAD_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **HTTP 429 rate limit errors during sync** — The client auto-throttles and retries with backoff; if it persists, re-run sync later or lower --concurrency.
- **sync returns no campaigns or empty tables** — Confirm SMARTLEAD_API_KEY is exported and valid: run smartlead-pp-cli doctor.
- **health or drift show stale numbers** — Offline commands read the local mirror; re-run smartlead-pp-cli sync to refresh it.
- **drift reports no history for a campaign** — drift needs at least two synced snapshots over time; run sync on a schedule so history accumulates.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**LeadMagic/smartlead-mcp-server**](https://github.com/LeadMagic/smartlead-mcp-server) — TypeScript (18 stars)
- [**LeadMagic/cold-email-cli**](https://github.com/LeadMagic/cold-email-cli) — TypeScript (7 stars)
- [**bcharleson/smartlead-cli**](https://github.com/bcharleson/smartlead-cli) — TypeScript (3 stars)
- [**smartlead-ai/API-Python-Library**](https://github.com/smartlead-ai/API-Python-Library) — Python
- [**jonathan-politzki/smartlead-mcp-server**](https://github.com/jonathan-politzki/smartlead-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
