# Linear CLI

Manage issues, projects, cycles, and teams via the Linear API with offline search and analytics.

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/project-management/linear/cmd/linear-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/linear-current).

## Authentication

Get your API key from [Linear Settings > API](https://linear.app/settings/api).

```bash
export LINEAR_API_KEY="lin_api_abc123..."
```

You can also persist this in your config file at `~/.config/linear-pp-cli/config.toml`:

```toml
api_key = "lin_api_abc123..."
```

To override the API base URL (for self-hosted or proxied setups):

```bash
export LINEAR_BASE_URL="https://your-proxy.example.com/graphql"
```

## Quick Start

```bash
# Check that credentials are working
linear-pp-cli doctor

# Sync issues, projects, teams, cycles, labels, and users locally
linear-pp-cli sync

# See your issues for today across all teams
linear-pp-cli today

# Look up a specific issue
linear-pp-cli issues ESP-1155

# List issues assigned to you (excludes completed/canceled by default)
linear-pp-cli issues list --assignee me

# Narrow to a team and state, output JSON
linear-pp-cli issues list --team ESP --state started --json

# Find stale issues that need attention
linear-pp-cli stale --days 14

# Search for potential duplicate issues
linear-pp-cli similar "login timeout"
```

## Finding Issues

`issues <ID>` fetches a single issue. Resolution order with the default `--data-source auto`:

1. local sqlite store, matched by identifier
2. live Linear GraphQL query (POST)
3. on live failure with a stale store, return the store miss as not found

`issues list` lists issues from the local store. Run `linear-pp-cli sync` first.

Filters compose with AND:

| Flag | Accepts | Notes |
|------|---------|-------|
| `--assignee` | `me`, UUID, display name, or email | `me` resolves the authenticated viewer via a live query |
| `--state` | `active` (default), `started`, `backlog`, `unstarted`, `completed`, `canceled`, `triage`, `all` | Matches `state.type`, so custom state names still work |
| `--team` | Team key (e.g. `ESP`) or UUID | Resolved against the local `teams` table |
| `--project` | Project name or UUID | Resolved against the local `projects` table |
| `--limit` | Integer | Defaults to 200 |
| `--json` | Flag | JSON output for piping to `jq` |

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`today`** - See all your issues for today across every team, sorted by priority and cycle deadline
- **`stale`** - Find issues that haven't been touched in weeks, grouped by team and project
- **`bottleneck`** - See which team members are overloaded and which issues are blocked before sprint planning
- **`similar`** - Find issues that might be duplicates using fuzzy text matching across titles and descriptions
- **`workload`** - Distribution of issues and estimates per team member with overload warnings
- **`velocity`** - Track sprint completion rates over time to spot team productivity trends

## Usage

```
Manage issues, projects, cycles, and teams via the Linear API with offline search and analytics

Usage:
  linear-pp-cli [command]

Available Commands:
  analytics                        Run analytics queries on locally synced data
  api                              Browse all API endpoints by interface name
  attachments                      Get a single attachment
  audit-entry-types                Get a single auditentrytype
  auth                             Manage authentication tokens
  bottleneck                       Find overloaded team members and blocked issues
  completion                       Generate the autocompletion script for the specified shell
  customers                        Get a single customer
  cycles                           Get a single cycle
  doctor                           Check CLI health
  documents                        Get a single document
  export                           Export data to JSONL or JSON for backup, migration, or analysis
  favorites                        Get a single favorite
  help                             Help about any command
  import                           Import data from JSONL file via API create/upsert calls
  initiatives                      Get a single initiative
  integrations                     Get a single integration
  issue-labels                     Get a single issuelabel
  issue-priority-values            Get a single issuepriorityvalue
  issue-relations                  Get a single issuerelation
  issues                           Get a single issue
  load                             Show workload distribution per assignee
  me                               Show current authenticated user
  organizations                    Get a single organization
  orphans                          Find items missing key fields like assignee or project
  projects                         Get a single project
  release-pipelines                Get a single releasepipeline
  releases                         Get a single release
  roadmaps                         Get a single roadmap
  similar                          Find potentially duplicate issues using fuzzy text search
  sql                              Run read-only SQL against the local store
  stale                            Find issues not updated in N days
  sync                             Sync Linear data to local SQLite store
  tail                             Stream live changes by polling the API at regular intervals
  teams                            Get a single team
  templates                        Get a single template
  today                            Show your issues for today across all teams
  users                            Get a single user
  velocity                         Show sprint velocity trends over recent cycles
  version                          Print version
  workflow                         Compound workflows that combine multiple API operations
  workload                         Show issue and estimate distribution per team member
```

## Commands

### Daily Workflow

| Command | Description |
|---------|-------------|
| `today` | Show your issues for today across all teams |
| `me` | Show current authenticated user |
| `stale` | Find issues not updated in N days |
| `orphans` | Find items missing key fields like assignee or project |
| `similar` | Find potentially duplicate issues using fuzzy text search |

### Team Analytics

| Command | Description |
|---------|-------------|
| `bottleneck` | Find overloaded team members and blocked issues |
| `workload` | Show issue and estimate distribution per team member |
| `load` | Show workload distribution per assignee |
| `velocity` | Show sprint velocity trends over recent cycles |
| `analytics` | Run analytics queries on locally synced data |
| `sql` | Run read-only SQL against the local store |

### Data Management

| Command | Description |
|---------|-------------|
| `sync` | Sync Linear data to local SQLite store |
| `export` | Export data to JSONL or JSON for backup, migration, or analysis |
| `import` | Import data from JSONL file via API create/upsert calls |
| `tail` | Stream live changes by polling the API at regular intervals |

### Resources

| Command | Description |
|---------|-------------|
| `issues` | Get a single issue |
| `projects` | Get a single project |
| `teams` | Get a single team |
| `cycles` | Get a single cycle |
| `initiatives` | Get a single initiative |
| `customers` | Get a single customer |
| `documents` | Get a single document |
| `releases` | Get a single release |
| `users` | Get a single user |
| `integrations` | Get, create, or delete integrations |
| `api` | Browse all API endpoints by interface name |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check CLI health |
| `auth` | Manage authentication tokens |
| `workflow` | Compound workflows that combine multiple API operations |
| `version` | Print version |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
linear-pp-cli today

# JSON for scripting and agents
linear-pp-cli today --json

# Filter to specific fields
linear-pp-cli issues abc123 --json --select id,title,state

# CSV for spreadsheets
linear-pp-cli stale --days 30 --csv

# Dry run - show the request without sending
linear-pp-cli issues abc123 --dry-run

# Compact mode - minimal fields for token efficiency
linear-pp-cli today --compact

# Agent mode - JSON + compact + no prompts in one flag
linear-pp-cli today --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - `echo '{"key":"value"}' | linear-pp-cli integrations create --stdin`
- **Cacheable** - GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set
- **Progress events** - paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add linear linear-pp-mcp -e LINEAR_API_KEY=<your-key>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "linear": {
      "command": "linear-pp-mcp",
      "env": {
        "LINEAR_API_KEY": "<your-key>"
      }
    }
  }
}
```

## Cookbook

```bash
# See what you need to work on today
linear-pp-cli today

# Find issues untouched for 2 weeks on the ENG team
linear-pp-cli stale --days 14 --team ENG

# Check who is overloaded before sprint planning
linear-pp-cli bottleneck --team ENG

# Find potential duplicate issues
linear-pp-cli similar "payment failed"

# Track velocity over the last 8 sprints
linear-pp-cli velocity --weeks 8

# Check workload balance across a team
linear-pp-cli workload --team ENG

# Run ad-hoc SQL against synced data
linear-pp-cli sql "SELECT identifier, title FROM issues WHERE priority = 1"

# Count issues by team
linear-pp-cli sql "SELECT team_id, count(*) as cnt FROM issues GROUP BY team_id"

# Export all issues for backup
linear-pp-cli export issues --format jsonl --output issues-backup.jsonl

# Stream changes in real time
linear-pp-cli tail --interval 10s | jq 'select(.type == "issue")'

# Find unassigned issues that need triage
linear-pp-cli orphans --limit 20

# Full re-sync from scratch
linear-pp-cli sync --full

# Pipe issue data to another tool
linear-pp-cli today --json | jq '.[].title'
```

## Health Check

```bash
$ linear-pp-cli doctor
Checking CLI health...

  Config file    ~/.config/linear-pp-cli/config.toml
  API key        set (env:LINEAR_API_KEY)
  Base URL       https://api.linear.app/graphql
  Connectivity   ok
  Local store    ~/.local/share/linear-pp-cli/store.db (synced)
```

## Configuration

Config file: `~/.config/linear-pp-cli/config.toml`

Environment variables:

| Variable | Description |
|----------|-------------|
| `LINEAR_API_KEY` | API key for authentication (required) |
| `LINEAR_BASE_URL` | Override the API base URL (default: `https://api.linear.app/graphql`) |
| `LINEAR_CONFIG` | Override the config file path |

## Troubleshooting

**Authentication errors (exit code 4)**
- Run `linear-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $LINEAR_API_KEY`
- Get a new key at [linear.app/settings/api](https://linear.app/settings/api)

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Use `linear-pp-cli sql "SELECT id, title FROM issues LIMIT 5"` to browse available items

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Use `--rate-limit 2` to cap requests per second
- If persistent, wait a few minutes and try again

**Sync errors**
- Run `linear-pp-cli sync --full` for a clean re-sync
- Check connectivity with `linear-pp-cli doctor`

**Empty results from analytics commands**
- Run `linear-pp-cli sync` first to populate the local store
- Check that your API key has access to the relevant teams

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**Finesssee/linear-cli**](https://github.com/Finesssee/linear-cli) - Rust
- [**schpet/linear-cli**](https://github.com/schpet/linear-cli) - Ruby
- [**czottmann/linearis**](https://github.com/czottmann/linearis) - TypeScript
- [**dorkitude/linctl**](https://github.com/dorkitude/linctl) - Go
- [**evangodon/linear-cli**](https://github.com/evangodon/linear-cli) - Go
- [**tacticlaunch/mcp-linear**](https://github.com/tacticlaunch/mcp-linear) - TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `linear-pp-cli profile save <name> --<flag> <value>`, then `linear-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.linear-pp-cli/feedback.jsonl`. When `LINEAR_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `LINEAR_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
