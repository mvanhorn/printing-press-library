# Wahoo Cloud API CLI

**Every Wahoo Cloud API capability, plus the local training database and offline analysis Wahoo's app never gives you.**

Sync your Wahoo rides, routes, plans, and power zones into a local SQLite mirror, then do what the app can't: back up every ride and FIT file, compute Fitness/Fatigue/Form and FTP progression, find a saved route by distance and elevation, and roll up any window of training. Agent-native output (--json/--agent/--select) makes it scriptable end to end.

Learn more at [Wahoo Cloud API](https://developers.wahooligan.com).

Created by [@stellato](https://github.com/stellato) (Greg Stellato).

## Install

The recommended path installs both the `wahoo-pp-cli` binary and the `pp-wahoo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install wahoo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install wahoo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install wahoo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install wahoo --agent claude-code
npx -y @mvanhorn/printing-press-library install wahoo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/wahoo/cmd/wahoo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wahoo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install wahoo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-wahoo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-wahoo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install wahoo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/wahoo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WAHOO_CLOUD_OAUTH2` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/devices/wahoo/cmd/wahoo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wahoo": {
      "command": "wahoo-pp-mcp",
      "env": {
        "WAHOO_CLOUD_OAUTH2": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth is OAuth 2.0 Authorization Code with PKCE. A Sandbox app works immediately against your own account with NO Wahoo approval — approval is only needed for higher rate limits or accessing other people's data. Register an app at https://developers.wahooligan.com/applications/new with the redirect URI http://localhost:8085/callback, pick your scopes, then set WAHOO_CLIENT_ID and WAHOO_CLIENT_SECRET (or pass --client-id/--client-secret) and run 'wahoo auth login' to authorize in your browser (use --port to change the callback port; sign in to Wahoo however you normally do, including Sign in with Google). Access tokens last 2 hours and refresh automatically (refresh tokens are single-use); you can also paste a pre-obtained token via WAHOO_CLOUD_OAUTH2. Revoke this app's access any time with 'wahoo permissions'. Run 'wahoo-pp-cli auth setup' for the full step-by-step.

## Quick Start

```bash
# Confirm config and rate-limit status; no auth needed.
wahoo doctor --dry-run

# Authorize in the browser (needs an approved Wahoo app).
wahoo auth login

# Pull your data into the local SQLite mirror.
wahoo sync --resources workouts,routes,power_zones

# Compute Fitness/Fatigue/Form from your rides.
wahoo load --days 90

# Download every ride record and FIT file locally.
wahoo backup --out ./wahoo-archive --full

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Own your data
- **`backup`** — Download every workout record plus its raw FIT file into a resumable local directory tree so you own a permanent backup.

  _Reach for this when the user wants their rides backed up or exported off Wahoo's cloud — the app has no bulk export._

  ```bash
  wahoo backup --out ./wahoo-archive --full
  ```

### Training analytics the app won't compute
- **`load`** — Compute CTL (Fitness), ATL (Fatigue), and TSB (Form) time series from your synced rides — the analysis TrainingPeaks charges for.

  _Pick this when the user asks how fit, fatigued, or fresh they are, or whether to push or rest this week._

  ```bash
  wahoo load --days 90 --agent
  ```
- **`digest`** — One-shot rollup of the last N days: ride count, distance, time, ascent, work, and load delta — pipeable to any tool.

  _Use for 'how much did I ride this week/month/year' (--days 365 covers year-in-review)._

  ```bash
  wahoo digest --days 7 --agent
  ```
- **`bests`** — All-time and per-period records for distance, ascent, average power, duration, and work from your ride summaries.

  _Pick this for PRs and bragging-rights records; note it uses ride summaries, not per-second streams._

  ```bash
  wahoo bests --metric power --agent
  ```
- **`ftp-history`** — Dated timeline of your FTP and critical power from power-zone records and sync snapshots, with watts/kg when your weight is known.

  _Use when the user wants to see whether their FTP is trending up over a training block._

  ```bash
  wahoo ftp-history --agent
  ```

### Offline queries over your library
- **`routes find`** — Filter your saved routes by distance band, climbing, and proximity to a point — query a route library the app only scrolls.

  _Pick this when the user wants 'a ~100km route under 1000m of climbing near here' from their own saved routes._

  ```bash
  wahoo routes find --distance 80-120 --max-ascent 1000 --agent
  ```

## Recipes


### Inspect a ride's key metrics for an agent

```bash
wahoo workouts get 12345 --agent --select id,name,workout_summary.distance_accum,workout_summary.power_bike_avg,workout_summary.work_accum
```

Narrows a deeply nested workout (with embedded summary) to just the fields an agent needs, avoiding tens of KB of payload.

### Back up an entire ride history

```bash
wahoo sync --resources workouts && wahoo backup --out ./wahoo-archive --full
```

Mirrors all workouts, then downloads each one's FIT file into a resumable local archive.

### Check form before a hard week

```bash
wahoo load --days 90 --agent
```

Returns the Fitness/Fatigue/Form series so an agent can advise whether to add intensity or rest.

### Pick a Saturday route

```bash
wahoo routes find --distance 90-110 --near 40.7,-74.0 --radius 25 --agent
```

Filters saved routes to ~90-110km within 25km of a point using stored geo and elevation fields.

## Usage

Run `wahoo-pp-cli --help` for the full command reference and flag list.

## Commands

### permissions

Revoke OAuth app access.

- **`wahoo-pp-cli permissions`** - Revoke App Access

### plans

Structured workout plans.

- **`wahoo-pp-cli plans create`** - Create Plan
- **`wahoo-pp-cli plans delete`** - Delete Plan
- **`wahoo-pp-cli plans get`** - Get Plan
- **`wahoo-pp-cli plans list`** - List Plans
- **`wahoo-pp-cli plans update`** - Update Plan

### power-zones

Cycling power training zones.

- **`wahoo-pp-cli power-zones create`** - Create Power Zones
- **`wahoo-pp-cli power-zones delete`** - Delete Power Zones
- **`wahoo-pp-cli power-zones get`** - Get Power Zones
- **`wahoo-pp-cli power-zones list`** - List Power Zones
- **`wahoo-pp-cli power-zones update`** - Update Power Zones

### routes

Navigation / course data backed by FIT files.

- **`wahoo-pp-cli routes create`** - Create Route
- **`wahoo-pp-cli routes delete`** - Delete Route
- **`wahoo-pp-cli routes get`** - Get Route
- **`wahoo-pp-cli routes list`** - List Routes
- **`wahoo-pp-cli routes update`** - Update Route

### user

Authenticated user profile.

- **`wahoo-pp-cli user get`** - Get Authenticated User
- **`wahoo-pp-cli user update`** - Update Authenticated User

### workout-file-uploads

Asynchronous FIT-file ingestion.

- **`wahoo-pp-cli workout-file-uploads create`** - Upload Workout FIT File
- **`wahoo-pp-cli workout-file-uploads get`** - Get Workout File Upload Status

### workouts

Workout records (CRUD + listing).

- **`wahoo-pp-cli workouts create`** - Create Workout
- **`wahoo-pp-cli workouts delete`** - Delete Workout
- **`wahoo-pp-cli workouts get`** - Get Workout
- **`wahoo-pp-cli workouts list`** - List Workouts
- **`wahoo-pp-cli workouts update`** - Update Workout


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
wahoo-pp-cli plans list

# JSON for scripting and agents
wahoo-pp-cli plans list --json

# Filter to specific fields
wahoo-pp-cli plans list --json --select id,name,status

# Dry run — show the request without sending
wahoo-pp-cli plans list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
wahoo-pp-cli plans list --agent
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
wahoo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/wahoo-cloud-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WAHOO_CLOUD_OAUTH2` | per_call | No | Optional: paste a pre-obtained OAuth access token to skip `auth login` (handy for MCP/CI). The primary path is `wahoo-pp-cli auth login`. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `wahoo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `wahoo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WAHOO_CLOUD_OAUTH2`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Invalid access token** — Run 'wahoo auth login' to reauthorize; access tokens expire after 2 hours and refresh tokens are single-use.
- **429 rate limited during sync** — Sandbox apps allow only 250 requests/day; pace with 'wahoo sync --max-pages N' or wait for the window to reset. FIT downloads (backup) are exempt.
- **auth login fails before approval** — Wahoo must approve your app first — register at developers.wahooligan.com/applications/new and wait for the pending status to clear.
- **load or bests shows no data** — Run 'wahoo sync --resources workouts' first; analysis commands read the local mirror.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wahoo-mcp**](https://github.com/armonge/wahoo-mcp) — Python (7 stars)
- [**wahoolib**](https://github.com/parkerhancock/wahoolib) — Python
- [**go-wahoo-cloud-api**](https://github.com/james-millner/go-wahoo-cloud-api) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
