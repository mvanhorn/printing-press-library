# ESPN CLI

Live scores, standings, news, and game history across 17 sports from ESPN

## Install

The recommended path installs both the `espn-pp-cli` binary and the `pp-espn` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install espn
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install espn --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/cmd/espn-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/espn-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-espn --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-espn --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-espn skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-espn. The skill defines how its required CLI can be installed.
```

## Authentication

The ESPN public API does not require authentication. No API key is needed.

If you need to point at a different ESPN API endpoint (e.g., a proxy or mirror):

```bash
export ESPN_BASE_URL="https://your-proxy.example.com/api"
```

## Quick Start

```bash
# Check that everything is working
espn-pp-cli doctor

# See all live scores across NFL, NBA, MLB, and NHL
espn-pp-cli today

# Sync game data locally for offline search
espn-pp-cli sync

# Search your synced data
espn-pp-cli search "Lakers"

# Check a team's win/loss streak
espn-pp-cli streak basketball nba --team LAL
```

## Unique Features

These capabilities aren't available in any other tool for this API.

- **`today`** -- See all live games across NFL, NBA, MLB, and NHL in one command
- **`search`** -- Search your game history by team name, matchup, or venue instantly
- **`sql`** -- Run arbitrary SQL queries against your local sports database
- **`streak`** -- See any team's current win/loss streak computed from game history
- **`rivals`** -- Compare historical matchup records between any two teams

## Usage

```
Live scores, standings, news, and game history across 17 sports from ESPN

Usage:
  espn-pp-cli [command]

Available Commands:
  api         Browse all API endpoints by interface name
  auth        Manage authentication tokens
  doctor      Check CLI health
  export      Export data to JSONL or JSON for backup, migration, or analysis
  import      Import data from JSONL file via API create/upsert calls
  load        Show workload distribution per assignee
  news        Get latest news articles for a sport and league
  orphans     Find items missing key fields like assignee or project
  rankings    Get current AP, Coaches, and CFP poll rankings for college sports
  recap       Game recap with box score and leaders
  rivals      Head-to-head record between two teams from synced data
  scoreboard  Get scoreboard for a sport and league with optional date filtering
  scores      Live scores and results for a sport and league
  search      Full-text search across synced events and news
  sql         Run read-only SQL queries against the local database
  stale       Find items with no updates in N days
  standings   Conference/division standings for a sport and league
  streak      Current win/loss streak for a team from synced data
  summary     Get detailed game summary including box score, leaders, scoring plays, odds, and win probability
  sync        Sync API data to local SQLite for offline search and analysis
  teams       Get past and upcoming schedule for a specific team
  today       Today's scores across all major sports
  watch       Live score updates for a game (polls every 30s)
  workflow    Compound workflows that combine multiple API operations
```

## Commands

### Live Scores

| Command | Description |
|---------|-------------|
| `today` | All live scores across NFL, NBA, MLB, NHL |
| `scores <sport> <league>` | Scores for a specific sport and league |
| `scoreboard <sport> <league>` | Full scoreboard with date filtering |
| `watch <sport> <league> --event <id>` | Live score updates (polls every 30s) |

### Game Data

| Command | Description |
|---------|-------------|
| `recap <sport> <league> --event <id>` | Box score, leaders, and scoring breakdown |
| `summary <sport> <league> --event <id>` | Full game summary with odds and win probability |
| `standings <sport> <league>` | Conference/division standings |
| `rankings <sport> <league>` | AP, Coaches, and CFP poll rankings |

### Teams

| Command | Description |
|---------|-------------|
| `teams <sport> <league> <team_id>` | Team schedule with past and upcoming games |
| `teams list <sport> <league>` | List all teams in a league |
| `teams get <sport> <league> <team_id>` | Team details including record and logos |

### News

| Command | Description |
|---------|-------------|
| `news <sport> <league>` | Latest headlines for a sport and league |

### Analytics (from synced data)

| Command | Description |
|---------|-------------|
| `streak <sport> <league> --team <abbr>` | Current win/loss streak for a team |
| `rivals <sport> <league> --teams <A,B>` | Head-to-head matchup record |
| `search <query>` | Full-text search across events and news |
| `sql <query>` | Raw SQL queries against the local database |

### Data Management

| Command | Description |
|---------|-------------|
| `sync` | Sync ESPN data to local SQLite |
| `export <resource>` | Export data to JSONL or JSON |
| `import <file>` | Import data from JSONL |
| `workflow archive` | Sync all resources for offline access |
| `workflow status` | Show local archive and sync state |

### Utilities

| Command | Description |
|---------|-------------|
| `doctor` | Check CLI health and API connectivity |
| `api` | Browse all API endpoints |
| `auth` | Manage authentication tokens |
| `load` | Workload distribution per assignee |
| `stale` | Items with no updates in N days |
| `orphans` | Items missing key fields |

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
espn-pp-cli scores football nfl

# JSON for scripting and agents
espn-pp-cli scores football nfl --json

# Filter to specific fields
espn-pp-cli scores football nfl --json --select matchup,away_team,home_team

# CSV output
espn-pp-cli standings football nfl --csv

# Compact mode for minimal token usage
espn-pp-cli today --compact

# Dry run — show the request without sending
espn-pp-cli news basketball nba --dry-run

# Agent mode — JSON + compact + no prompts in one flag
espn-pp-cli today --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** -- never prompts, every input is a flag
- **Pipeable** -- `--json` output to stdout, errors to stderr
- **Filterable** -- `--select id,name` returns only fields you need
- **Previewable** -- `--dry-run` shows the request without sending
- **Confirmable** -- `--yes` for explicit confirmation of destructive actions
- **Cacheable** -- GET responses cached for 5 minutes, bypass with `--no-cache`
- **Agent-safe by default** -- no colors or formatting unless `--human-friendly` is set
- **Progress events** -- paginated commands emit NDJSON events to stderr in default mode

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add espn espn-pp-mcp
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "espn": {
      "command": "espn-pp-mcp"
    }
  }
}
```

## Cookbook

```bash
# See all live NFL scores
espn-pp-cli scores football nfl

# Check NBA standings
espn-pp-cli standings basketball nba

# Get NFL scores from a specific date
espn-pp-cli scoreboard football nfl --dates 20250112

# Watch a game live (polls every 30s)
espn-pp-cli watch football nfl --event 401547417

# Get a game recap with box score
espn-pp-cli recap basketball nba --event 401584793

# Check the Chiefs' current streak
espn-pp-cli streak football nfl --team KC

# Head-to-head: Yankees vs Red Sox
espn-pp-cli rivals baseball mlb --teams NYY,BOS

# College football rankings
espn-pp-cli rankings football college-football

# Sync data for offline queries
espn-pp-cli sync

# SQL query against synced data
espn-pp-cli sql "SELECT short_name, home_score, away_score FROM events WHERE league='nfl' LIMIT 10"

# Search synced games and news
espn-pp-cli search "touchdown" --sport football

# Export NFL data as JSONL
espn-pp-cli export football/nfl/scoreboard --format jsonl

# Pipe today's scores to jq
espn-pp-cli today --json | jq '.NBA[] | select(.status == "in") | .matchup'
```

## Health Check

```bash
espn-pp-cli doctor
```

```
  OK Config: ok
  WARN Auth: not required
  OK API: reachable
  config_path: ~/.config/espn-pp-cli/config.toml
  base_url: https://site.api.espn.com/apis/site/v2/sports
  version: 0.1.0
```

## Configuration

Config file: `~/.config/espn-pp-cli/config.toml`

Environment variables:

| Variable | Description |
|----------|-------------|
| `ESPN_CONFIG` | Override config file path |
| `ESPN_BASE_URL` | Override API base URL (for proxies or self-hosted mirrors) |
| `NO_COLOR` | Disable colored output (standard) |

## Troubleshooting

**Authentication errors (exit code 4)**
- The ESPN public API generally does not require authentication
- Run `espn-pp-cli doctor` to check connectivity

**Not found errors (exit code 3)**
- Check that the sport and league names are correct (e.g., `football nfl`, `basketball nba`)
- Use `teams list <sport> <league>` to verify team IDs

**Rate limit errors (exit code 7)**
- The CLI auto-retries with exponential backoff
- Use `--rate-limit 2` to cap requests per second
- If persistent, wait a few minutes and try again

**No local data errors**
- Run `espn-pp-cli sync` to populate the local database
- Use `--data-source live` to bypass local data and query the API directly

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**espn-api**](https://github.com/cwendt94/espn-api) -- Python (890 stars)
- [**Public-ESPN-API**](https://github.com/pseudo-r/Public-ESPN-API) -- Python (443 stars)
- [**fantasy-football-metrics**](https://github.com/uberfastman/fantasy-football-metrics-weekly-report) -- Python (223 stars)
- [**mcp_espn_ff**](https://github.com/KBThree13/mcp_espn_ff) -- Python (30 stars)
- [**sportly**](https://github.com/pseudo-r/sportly) -- Python (2 stars)
- [**espn-mcp**](https://github.com/Left-Coast-Tech/espn-mcp) -- TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

<!-- pr-218-features -->
## Agent workflow features

This CLI was patched to add these agent-workflow capabilities (see [`printing-press patch`](https://github.com/mvanhorn/cli-printing-press/pull/221)):

- **Named profiles** — save a set of flags under a name and reuse them: `espn-pp-cli profile save <name> --<flag> <value>`, then `espn-pp-cli --profile <name> <command>`. Flag precedence: explicit flag > env var > profile > default.
- **`--deliver`** — route command output to a sink other than stdout. Values: `file:<path>` writes atomically via tmp+rename; `webhook:<url>` POSTs as JSON (or NDJSON with `--compact`).
- **`feedback`** — record in-band feedback about the CLI. Entries append as JSON lines to `~/.espn-pp-cli/feedback.jsonl`. When `ESPN_FEEDBACK_ENDPOINT` is set and either `--send` is passed or `ESPN_FEEDBACK_AUTO_SEND=true`, the entry is also POSTed upstream.
