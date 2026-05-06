# HubSpot CLI

Manage HubSpot CRM contacts, companies, deals, tickets, engagements, pipelines, and associations with offline search and pipeline analytics.

Learn more at [HubSpot Developers](https://developers.hubspot.com/docs/api).

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/cmd/hubspot-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hubspot-current).

## Authentication

Get a private app access token from your [HubSpot account settings](https://app.hubspot.com/private-apps/):

```bash
# Option 1: environment variable
export HUBSPOT_ACCESS_TOKEN="pat-na1-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

# Option 2: store in config file
hubspot-pp-cli auth set-token pat-na1-xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

You can also use `HUBSPOT_PRIVATE_APP_TOKEN` as an alternative environment variable.

For self-hosted or EU datacenter instances, override the base URL:

```bash
export HUBSPOT_BASE_URL="https://api.hubapi.eu"
```

## Quick Start

```bash
# 1. Verify your credentials
hubspot-pp-cli doctor

# 2. Sync CRM data locally for fast offline search
hubspot-pp-cli sync

# 3. See which deals are stuck with no engagement
hubspot-pp-cli deals stale --days 21

# 4. Check pipeline velocity to find bottlenecks
hubspot-pp-cli deals velocity --weeks 12

# 5. Search across all synced objects
hubspot-pp-cli search "acme corp"
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`deals velocity`** - See how fast deals move through each pipeline stage with conversion rates and bottleneck detection.
- **`deals stale`** - Find deals stuck in a stage with no recent engagement before they go cold.
- **`deals coverage`** - Find open deals where associated contacts have no recent engagement.
- **`contacts engagement`** - See engagement frequency and gaps for every contact across calls, emails, meetings, and tasks.
- **`owners workload`** - See which team members are overloaded with open deals, tickets, and overdue tasks before assigning more.

## Commands

### CRM Objects

| Command | Description |
|---------|-------------|
| `contacts` | List, create, search, update, and delete contacts |
| `companies` | List, create, search, update, and delete companies |
| `deals` | List, create, search, update, and delete deals |
| `tickets` | List, create, search, update, and delete tickets |

### Engagements

| Command | Description |
|---------|-------------|
| `calls` | List, create, and delete logged phone calls |
| `emails` | List, create, and delete logged emails |
| `meetings` | List, create, and delete meetings |
| `notes` | List, create, and delete notes |
| `tasks` | List, create, update, and delete tasks |

### Pipeline Analytics

| Command | Description |
|---------|-------------|
| `deals velocity` | Analyze deal stage durations and conversion rates |
| `deals stale` | Find deals stuck in a stage past a threshold |
| `deals coverage` | Find open deals with low engagement coverage |
| `contacts engagement` | Score contact engagement across activity types |
| `owners workload` | Cross-entity workload analysis per owner |

### Relationships and Properties

| Command | Description |
|---------|-------------|
| `associations` | List associations between CRM objects |
| `lists` | Get contact lists and their members |
| `owners` | List CRM record owners |
| `pipelines` | List deal pipelines and their stages |
| `properties` | List, create, and delete CRM object properties |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check CLI health, credentials, and API connectivity |
| `auth` | Manage authentication tokens |
| `sync` | Sync API data to local SQLite for offline search |
| `search` | Full-text search across synced data or live API |
| `export` | Export data to JSONL or JSON for backup |
| `import` | Import data from JSONL via API create/upsert |
| `tail` | Stream live changes by polling the API |
| `analytics` | Run count and group-by queries on synced data |
| `workflow` | Compound workflows (archive, status) |
| `api` | Browse all API endpoints by interface name |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hubspot-pp-cli contacts

# JSON for scripting and agents
hubspot-pp-cli contacts --json

# Filter to specific fields
hubspot-pp-cli contacts --json --select id,firstname,lastname,email

# CSV for spreadsheets
hubspot-pp-cli deals --csv

# Compact mode for minimal token usage
hubspot-pp-cli deals --compact

# Dry run - show the request without sending
hubspot-pp-cli contacts create --email test@example.com --dry-run

# Agent mode - JSON + compact + no prompts in one flag
hubspot-pp-cli deals --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | hubspot-pp-cli contacts create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add hubspot hubspot-pp-mcp -e HUBSPOT_ACCESS_TOKEN=<your-token>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hubspot": {
      "command": "hubspot-pp-mcp",
      "env": {
        "HUBSPOT_ACCESS_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Cookbook

```bash
# Create a contact with full details
hubspot-pp-cli contacts create --email jane@acme.com --firstname Jane --lastname Doe --company "Acme Inc"

# Create a deal in a specific pipeline stage
hubspot-pp-cli deals create --dealname "Acme Enterprise" --amount 50000 --pipeline default --dealstage qualifiedtobuy

# Search for contacts at a company
hubspot-pp-cli contacts search --query "acme"

# Find stale deals older than 3 weeks
hubspot-pp-cli deals stale --days 21 --json

# Analyze pipeline velocity over the last quarter
hubspot-pp-cli deals velocity --weeks 12 --json

# Check engagement coverage on open deals
hubspot-pp-cli deals coverage --pipeline default

# See which owners are overloaded
hubspot-pp-cli owners workload --limit 10

# Score contact engagement over 60 days
hubspot-pp-cli contacts engagement --days 60 --limit 20 --json

# Export all contacts as JSONL for backup
hubspot-pp-cli export contacts --format jsonl --output contacts-backup.jsonl

# Sync then search offline
hubspot-pp-cli sync && hubspot-pp-cli search "enterprise" --data-source local

# Stream deal changes in real time
hubspot-pp-cli tail deals --interval 30s | jq 'select(.properties.dealstage == "closedwon")'

# List associations between a contact and their companies
hubspot-pp-cli associations contacts 12345 companies

# View pipeline stages
hubspot-pp-cli pipelines stages default

# Analytics: group deals by stage
hubspot-pp-cli analytics --type deals --group-by dealstage --limit 10
```

## Health Check

```bash
hubspot-pp-cli doctor
```

```
  OK Config: ok
  FAIL Auth: not configured
  OK API: reachable
  config_path: ~/.config/hubspot-pp-cli/config.json
  base_url: https://api.hubapi.com
  version: 3.0.0
  hint: export HUBSPOT_ACCESS_TOKEN=<your-key>
```

## Configuration

Config file: `~/.config/hubspot-pp-cli/config.json`

Environment variables:
- `HUBSPOT_ACCESS_TOKEN` - OAuth2 or private app access token (primary)
- `HUBSPOT_PRIVATE_APP_TOKEN` - Private app token (alternative)
- `HUBSPOT_BASE_URL` - API base URL override (default: `https://api.hubapi.com`)
- `HUBSPOT_CONFIG` - Custom config file path

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `hubspot-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HUBSPOT_ACCESS_TOKEN`
- Ensure your token has the required scopes for the operation

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Use `--rate-limit 5` to cap requests per second
- If persistent, wait a few minutes and try again

**Stale local data**
- Run `hubspot-pp-cli sync --full` for a complete resync
- Use `--data-source live` to bypass local cache

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hubspot-cli (bcharleson)**](https://github.com/bcharleson/hubspot-cli) - TypeScript
- [**HubSpot MCP Server (official)**](https://developers.hubspot.com/mcp) - TypeScript
- [**mcp-hubspot (peakmojo)**](https://github.com/peakmojo/mcp-hubspot) - Python
- [**@hubspot/api-client**](https://github.com/HubSpot/hubspot-api-nodejs) - JavaScript
- [**hubspot-api-client (Python)**](https://github.com/HubSpot/hubspot-api-python) - Python
- [**go-hubspot (clarkmcc)**](https://github.com/clarkmcc/go-hubspot) - Go
- [**hubspot-cli (official CMS)**](https://github.com/HubSpot/hubspot-cli) - JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `hubspot-pp-cli profile save <name> --<flag> <value>`, then `hubspot-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.hubspot-pp-cli/feedback.jsonl`. When `HUBSPOT_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `HUBSPOT_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
