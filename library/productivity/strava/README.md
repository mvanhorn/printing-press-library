# Strava CLI

**Every Strava feature, plus offline analytics — training load, power curves, zone time, and segment progression — that the website can't show you.**

strava-pp-cli syncs your entire Strava history to a local SQLite database, then unlocks the analytics your training actually requires: CTL/ATL training load, power curve, zone time distribution, aerobic decoupling, and segment effort progression. Everything works offline, composes with jq, and speaks JSON natively for AI coaching agents.

Printed by [@azaaron](https://github.com/azaaron) (azaaron).

## Install

The recommended path installs both the `strava-pp-cli` binary and the `pp-strava` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install strava
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install strava --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install strava --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install strava --agent claude-code
npx -y @mvanhorn/printing-press install strava --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/strava-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-strava --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-strava --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-strava skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-strava. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/strava-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `STRAVA_STRAVA_OAUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "strava": {
      "command": "strava-pp-mcp",
      "env": {
        "STRAVA_STRAVA_OAUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Strava uses OAuth2. Run `strava-pp-cli auth login` to open the browser authorization page — the CLI handles the local callback, token exchange, and storage automatically. Set STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET before running. Tokens are refreshed automatically; run `auth refresh` to force a refresh.

## Quick Start

```bash
# Verify credentials and API connectivity (run auth login first if not authenticated)
strava-pp-cli doctor


# Verify credentials and check API connectivity
strava-pp-cli doctor


# Mirror your Strava data to a local SQLite database
strava-pp-cli sync


# See your CTL/ATL/TSB training load timeline
strava-pp-cli training load --weeks 12


# Compute your power curve from synced stream data
strava-pp-cli athlete power-curve --since 2025-01-01


# Track your progression on a specific segment over time
strava-pp-cli segments progress 229781

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`segments progress`** — See your entire effort history on a segment — date, time, avg power, avg HR, delta from PR — so you can track if you're actually improving.

  _Use this when an agent needs to assess whether an athlete is progressing on a target training segment over a season._

  ```bash
  strava-pp-cli segments progress 229781 --json --select start_date,elapsed,avg_watts,delta_pr_seconds
  ```
- **`training load`** — See your Chronic Training Load, Acute Training Load, and Training Stress Balance as sparklines so you can spot overtraining or undertaper before a race.

  _Use when an agent needs to assess an athlete's fitness/fatigue state and readiness for an upcoming event._

  ```bash
  strava-pp-cli training load --weeks 12 --ftp 285 --agent
  ```
- **`training zones`** — See how many minutes per week you actually spent in each heart rate or power zone, so you can tell if your training distribution matches your plan.

  _Use when evaluating whether a training block was executed as prescribed (polarized, sweet-spot, base)._

  ```bash
  strava-pp-cli training zones --weeks 8 --type Run --zone-type heartrate --agent
  ```
- **`athlete power-curve`** — See your best mean power for every standard duration (1s to 60min) so you can identify strengths, weaknesses, and fitness changes across seasons.

  _Use when an agent needs to characterize a cyclist's physiological profile or compare peak power across training blocks._

  ```bash
  strava-pp-cli athlete power-curve --since 2025-01-01 --weight 75 --agent
  ```
- **`activities drift`** — Measure aerobic decoupling in an activity — the ratio of HR rise to pace drop in the second half — to assess aerobic fitness without lab testing.

  _Use when an agent needs to identify which long runs or rides showed aerobic decoupling, indicating the athlete was above their aerobic threshold._

  ```bash
  strava-pp-cli activities drift --min-duration 45m --since 2025-01-01 --threshold 5 --agent
  ```
- **`segments kom-gap`** — See exactly how far you are from the KOM on each starred segment, ranked by the gap you're most likely to close.

  _Use when an agent needs to surface the most achievable KOM targets for a training plan or pre-ride goal setting._

  ```bash
  strava-pp-cli segments kom-gap --top 10 --agent
  ```

### Agent-native plumbing
- **`activities bulk-update`** — Update gear, name template, or description across hundreds of activities at once with a preview-before-commit safety net.

  _Use when an agent needs to mass-migrate a gear assignment after equipment replacement or retroactively organize a training block's activities._

  ```bash
  strava-pp-cli activities bulk-update --type Ride --after 2024-01-01 --set-gear b12345678 --dry-run
  ```
- **`gear status`** — See total mileage on each shoe and bike, your configured replacement threshold, and an estimated retirement date based on your recent usage rate.

  _Use when an agent needs to check whether any gear is approaching retirement before an important race._

  ```bash
  strava-pp-cli gear status --threshold shoes=500mi --agent
  ```

## Usage

Run `strava-pp-cli --help` for the full command reference and flag list.

## Commands

### activities

Manage activities

- **`strava-pp-cli activities create-activity`** - Creates a manual activity for an athlete, requires activity:write scope.
- **`strava-pp-cli activities get-activity-by-id`** - Returns the given activity that is owned by the authenticated athlete. Requires activity:read for Everyone and Followers activities. Requires activity:read_all for Only Me activities.

We strongly encourage you to display the appropriate attribution that identifies Garmin as the data source and the device name in your application. Please see example below from VeloViewer (that provides an attribution for a Garmin Forerunner device).

![Attribution](/images/device-attribution-image.png)
- **`strava-pp-cli activities update-activity-by-id`** - Updates the given activity that is owned by the authenticated athlete. Requires activity:write. Also requires activity:read_all in order to update Only Me activities

### athlete

Manage athlete

- **`strava-pp-cli athlete get-logged-in`** - Returns the currently authenticated athlete. Tokens with profile:read_all scope will receive a detailed athlete representation; all others will receive a summary representation.
- **`strava-pp-cli athlete get-logged-in-activities`** - Returns the activities of an athlete for a specific identifier. Requires activity:read. Only Me activities will be filtered out unless requested by a token with activity:read_all.
- **`strava-pp-cli athlete get-logged-in-clubs`** - Returns a list of the clubs whose membership includes the authenticated athlete.
- **`strava-pp-cli athlete get-logged-in-zones`** - Returns the the authenticated athlete's heart rate and power zones. Requires profile:read_all.
- **`strava-pp-cli athlete update-logged-in`** - Update the currently authenticated athlete. Requires profile:write scope.

### athletes

Manage athletes


### clubs

Manage clubs

- **`strava-pp-cli clubs <id>`** - Returns a given a club using its identifier.

### gear

Manage gear

- **`strava-pp-cli gear <id>`** - Returns an equipment using its identifier.

### routes

Manage routes

- **`strava-pp-cli routes <id>`** - Returns a route using its identifier. Requires read_all scope for private routes.

### segment-efforts

Manage segment efforts

- **`strava-pp-cli segment-efforts get-by-id`** - Returns a segment effort from an activity that is owned by the authenticated athlete. Requires subscription.
- **`strava-pp-cli segment-efforts get-efforts-by-segment-id`** - Returns a set of the authenticated athlete's segment efforts for a given segment.  Requires subscription.

### segments

Manage segments

- **`strava-pp-cli segments explore`** - Returns the top 10 segments matching a specified query.
- **`strava-pp-cli segments get-by-id`** - Returns the specified segment. read_all scope required in order to retrieve athlete-specific segment information, or to retrieve private segments.
- **`strava-pp-cli segments get-logged-in-athlete-starred`** - List of the authenticated athlete's starred segments. Private segments are filtered out unless requested by a token with read_all scope.

### uploads

Manage uploads

- **`strava-pp-cli uploads create`** - Uploads a new data file to create an activity from. Requires activity:write scope.
- **`strava-pp-cli uploads get-by-id`** - Returns an upload for a given identifier. Requires activity:write scope.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
strava-pp-cli activities create-activity --name example-resource

# JSON for scripting and agents
strava-pp-cli activities create-activity --name example-resource --json

# Filter to specific fields
strava-pp-cli activities create-activity --name example-resource --json --select id,name,sport_type

# Dry run — show the request without sending
strava-pp-cli activities create-activity --name example-resource --dry-run

# Agent mode — JSON + compact + no prompts in one flag
strava-pp-cli activities create-activity --name example-resource --agent
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
strava-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/strava-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `STRAVA_STRAVA_OAUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `strava-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $STRAVA_STRAVA_OAUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login fails with redirect_uri mismatch** — Set STRAVA_REDIRECT_URI to the value configured in your Strava API application settings (default: http://localhost:8421/callback)
- **HTTP 429 during sync** — Strava rate-limits at 200 requests/15min. Run sync --slow to add a 1s delay between requests, or wait for the 15-minute window to reset
- **power-curve returns no data** — Power data requires sync with --streams flag: run sync --streams to fetch per-second watts data. Requires activities recorded with a power meter
- **training load shows 0 TSS** — TSS computation uses Strava's suffer_score by default. For power-based TSS supply --ftp <watts> and run sync --streams first
- **bulk-update hits rate limit mid-run** — Re-run with --rate-limit-sleep 4 to spread requests across the 15-min window. The command is idempotent — already-updated activities are skipped

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**liskin/strava-offline**](https://github.com/liskin/strava-offline) — Python (1000 stars)
- [**hozn/stravalib**](https://github.com/hozn/stravalib) — Python (600 stars)
- [**eddmann/strava-cli**](https://github.com/eddmann/strava-cli) — Python (400 stars)
- [**eddmann/strava-mcp**](https://github.com/eddmann/strava-mcp) — Python (300 stars)
- [**nickel/strava-v3**](https://github.com/nickel/strava-v3) — JavaScript (300 stars)
- [**bwilczynski/strava-cli**](https://github.com/bwilczynski/strava-cli) — Python (200 stars)
- [**dlenski/stravacli**](https://github.com/dlenski/stravacli) — Python (100 stars)
- [**Guutong/strava-mcp-kit**](https://github.com/Guutong/strava-mcp-kit) — TypeScript (50 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
