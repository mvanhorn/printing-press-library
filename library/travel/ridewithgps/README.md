# Ride with GPS CLI

**Your whole Ride with GPS library, offline — bulk export, gear mileage, and ride analytics no other tool ships.**

A fast, scriptable, agent-native CLI for Ride with GPS with a local SQLite mirror of your routes and trips. Bulk-export everything (with cue sheets) to GPX/TCX/CSV for your bike computer with 'export', pull an organized ride's routes with 'event-routes', track per-bike mileage with 'gear', see this month's climbing with 'stats', surface all-time bests with 'records', and clean a junk-drawer library with 'dedup' and 'audit' — none of which the web app or any existing tool does.

Learn more at [Ride with GPS](https://ridewithgps.com/api).

Created by [@stellato](https://github.com/stellato) (Greg Stellato).

## Install

The recommended path installs both the `ridewithgps-pp-cli` binary and the `pp-ridewithgps` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ridewithgps
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ridewithgps --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ridewithgps --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ridewithgps --agent claude-code
npx -y @mvanhorn/printing-press-library install ridewithgps --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/cmd/ridewithgps-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ridewithgps-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ridewithgps --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ridewithgps --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ridewithgps --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ridewithgps --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ridewithgps-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `RIDEWITHGPS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/cmd/ridewithgps-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ridewithgps": {
      "command": "ridewithgps-pp-mcp",
      "env": {
        "RIDEWITHGPS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Ride with GPS uses two credentials: a client API key and a per-user auth token, sent as the x-rwgps-api-key and x-rwgps-auth-token headers. Create an API client under your account's developers tab for the API key, then set RIDEWITHGPS_API_KEY and RIDEWITHGPS_AUTH_TOKEN. To get your auth token from your email and password, run 'auth-tokens --user-email <you> --user-password <pw>', which returns it. The v1 API does not accept browser cookies; run 'doctor' to verify.

## Quick Start

```bash
# health check — confirms credentials and API reachability without writing anything
ridewithgps-pp-cli doctor --dry-run

# mirror your routes and trips into the local SQLite store
ridewithgps-pp-cli sync --resources routes,trips --full

# this month's distance, elevation, and moving time from the local mirror
ridewithgps-pp-cli stats --period month

# bulk-export every route to GPX — the thing the web app won't do
ridewithgps-pp-cli export --type routes --format gpx --out ./gpx

# per-bike mileage rolled up from your logged rides
ridewithgps-pp-cli gear

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Routes onto your bike computer
- **`export`** — Export many routes or trips to GPX, TCX, CSV, or KML files in one command — with cue sheets included, ready to side-load onto a Garmin, Wahoo, or Hammerhead. The bulk export Ride with GPS itself won't do.

  _Reach for this when a user wants routes onto a bike computer or rides as files on disk — one command instead of clicking export per route in the web app._

  ```bash
  ridewithgps-pp-cli export --type routes --format gpx --out ./gpx
  ```
- **`event-routes`** — List the routes attached to an event (group ride, gran fondo, brevet) and optionally export them straight to files for your bike computer.

  _Reach for this when a user has signed up for an organized ride and wants that event's official routes on their head unit._

  ```bash
  ridewithgps-pp-cli event-routes 12345678 --export gpx --out ./fondo
  ```

### Ride analytics that compound
- **`gear`** — Per-bike accumulated mileage from your logged rides, plus maintenance-due flags against wear thresholds.

  _Reach for this to answer 'how many miles since my last chain swap' or 'which bike needs service' without a spreadsheet._

  ```bash
  ridewithgps-pp-cli gear --due-km 4000 --agent
  ```
- **`stats`** — Time-windowed distance, elevation, and moving-time totals with activity-type breakdowns over your logged rides.

  _Reach for this for 'how far did I ride this month' or 'how much have I climbed this year' — period totals, not single efforts._

  ```bash
  ridewithgps-pp-cli stats --period month --agent
  ```
- **`records`** — All-time best efforts — longest ride, most climbing, fastest average speed, biggest power — from your trip metrics.

  _Reach for this for 'what's my longest ride' or 'my biggest climbing day' — top-N extremes, not period sums._

  ```bash
  ridewithgps-pp-cli records --metric elevation --top 10 --agent
  ```

### Library hygiene
- **`dedup`** — Find near-duplicate routes by distance and start/end geometry, and optionally delete the extras.

  _Reach for this to clean a route library full of near-identical versions of the same loop._

  ```bash
  ridewithgps-pp-cli dedup --threshold 100 --agent
  ```
- **`audit`** — Flag routes that are stale, private, or incomplete (missing name, description, or distance) — catalog hygiene across your library.

  _Reach for this before an event season to find routes that are stale, private when they should be shared, or missing details._

  ```bash
  ridewithgps-pp-cli audit --checks stale,private --agent
  ```

## Recipes


### Bulk-export the whole route library to GPX for your head unit

```bash
ridewithgps-pp-cli export --type routes --format gpx --out ./gpx
```

Writes one GPX file per route (with cue sheets) from the local mirror — ready to side-load onto a Garmin or Wahoo.

### Get an event's routes onto your bike computer

```bash
ridewithgps-pp-cli event-routes 12345678 --export gpx --out ./fondo
```

Pulls the routes attached to an organized ride and writes them as files for your head unit.

### Find and remove duplicate routes

```bash
ridewithgps-pp-cli dedup --threshold 100 --apply
```

Clusters near-identical routes within 100m and deletes the extras (omit --apply to preview).

### Check which bikes are due for service

```bash
ridewithgps-pp-cli gear --due-km 4000
```

Flags bikes past 4000km of accumulated trip mileage.

### Agent: pull a route's cue sheet with narrow fields

```bash
ridewithgps-pp-cli routes get 12345678 --agent --select route.name,route.distance,route.course_points.n,route.course_points.d
```

Route detail is deeply nested and large; --select narrows to cue-sheet names and distances so an agent does not parse tens of KB of track points.

### Agent: this year's training totals

```bash
ridewithgps-pp-cli stats --period year --agent
```

Returns structured yearly distance/elevation/time totals for downstream reasoning.

## Usage

Run `ridewithgps-pp-cli --help` for the full command reference and flag list.

## Commands

### auth_tokens

Manage auth tokens

- **`ridewithgps-pp-cli auth-tokens`** - Creates a new authentication token for API access using user credentials.

### collections

Manage collections

- **`ridewithgps-pp-cli collections get`** - Returns detailed information about a specific collection
- **`ridewithgps-pp-cli collections get-pinned`** - Returns the authenticated user's pinned collection
- **`ridewithgps-pp-cli collections list`** - Returns a paginated list of collections for the authenticated user.

Supports filtering by `name` and `visibility`.

### events

Manage events

- **`ridewithgps-pp-cli events create`** - Creates a new event for the authenticated user.

**Note**: The `organizers`, `logo`, and `banner` fields are supported only for organization-owned events. They are not supported for user-owned events.

**Multipart uploads**: To upload logo and banner images, send a request with `Content-Type: multipart/form-data` using the following fields:
- `event[name]` - Event name
- `event[description]` - Event description
- `event[logo]` - Logo image (binary)
- `event[banner]` - Banner image (binary)
- etc.
- **`ridewithgps-pp-cli events delete`** - Deletes an event owned by the authenticated user
- **`ridewithgps-pp-cli events get`** - Returns detailed information about a specific event
- **`ridewithgps-pp-cli events list`** - Returns a paginated list of events accessible to the authenticated user.

Supports filtering by `name` and `visibility`.
- **`ridewithgps-pp-cli events update`** - Updates an existing event owned by the authenticated user.

**Note**: The `organizers`, `logo`, and `banner` fields are supported only for organization-owned events. They are not supported for user-owned events.

**Multipart uploads**: To upload logo and banner images, send a request with `Content-Type: multipart/form-data` using the following fields:
- `event[name]` - Event name
- `event[description]` - Event description
- `event[logo]` - Logo image (binary)
- `event[banner]` - Banner image (binary)
- etc.

### members

Manage members

- **`ridewithgps-pp-cli members get-club`** - Returns detailed information about a specific club member
- **`ridewithgps-pp-cli members list-club`** - Returns a paginated list of members for the authenticated organization account.

This endpoint is only available to organization accounts.

Supports filtering by `name` and `email`.
- **`ridewithgps-pp-cli members update-club`** - Updates a club member's permissions and status.

Available attributes:
- `active`: Whether the member is active
- `admin`: Whether the member is an admin (grants all permissions)
- `manages_routes`: Permission to manage routes
- `manages_members`: Permission to manage members
- `manages_billing`: Permission to manage billing

### points_of_interest

Manage points of interest

- **`ridewithgps-pp-cli points-of-interest associate-point-of-interest-with-route`** - Associates a point of interest with a specific route

**Note**: This endpoint is available for organizations only.
- **`ridewithgps-pp-cli points-of-interest create-point-of-interest`** - Create a new point of interest in the authenticated organization's library.

**Note**: This endpoint is available for organizations only.
- **`ridewithgps-pp-cli points-of-interest delete-point-of-interest`** - Deletes a point of interest owned by the authenticated organization.

**Note**: This endpoint is available for organizations only.
- **`ridewithgps-pp-cli points-of-interest disassociate-point-of-interest-from-route`** - Removes the association between a point of interest and a specific route

**Note**: This endpoint is available for organizations only.
- **`ridewithgps-pp-cli points-of-interest get-point-of-interest`** - Returns detailed information about a specific point of interest.

**Note**: This endpoint is available for organizations only.
- **`ridewithgps-pp-cli points-of-interest list`** - Returns a paginated list of points of interest for the authenticated organization.

**Note**: This endpoint is available for organizations only.
- **`ridewithgps-pp-cli points-of-interest update-point-of-interest`** - Updates an existing point of interest in the authenticated organization's library.

**Note**: This endpoint is available for organizations only.

### routes

Manage routes

- **`ridewithgps-pp-cli routes delete`** - Deletes a route owned by the authenticated user
- **`ridewithgps-pp-cli routes get`** - Returns detailed information about a specific route
- **`ridewithgps-pp-cli routes get-polyline`** - Returns an encoded polyline string for the specified route.

The polyline is a simplified representation of the route geometry, suitable for displaying on a map.
- **`ridewithgps-pp-cli routes list`** - Returns a paginated list of routes for the authenticated user.

Supports filtering by `name`, `visibility`, `distance_min`, `distance_max`, `elevation_gain_min`, `elevation_gain_max` and `archived` status.

### sync-json

Manage sync json

- **`ridewithgps-pp-cli sync-json`** - Returns a list of items (routes and/or trips) that the user has interacted with since the given datetime.

The sync endpoint is convenient to maintain a remote copy of a user's library of trips and/or routes.

### Actions

Items can have the following actions:
* `created` - The user created the item
* `updated` - The user updated the item
* `deleted` - The user deleted the item
* `added` - The user added the item to the specified collection (e.g., pinned)
* `removed` - The user removed the item from the specified collection

For `added` and `removed` actions, `collection` is the target collection where the action was taken. It is currently always the collection of pinned items for the authenticated user, but we might expose other collections in the future.

### Sync Workflow

This endpoint is optimized for performance (we use a similar setup for syncing our own mobile apps), and can return the entire library for the user by using a `since` value well in the past (for example `1970-01-01`).

The following workflow is suitable to maintain your copy of a user's library:

1. When the user finalizes the OAuth workflow, get the current content of their library at `GET https://ridewithgps.com/api/v1/sync.json?since=1970-01-01`
2. Download and integrate in your system every item in the response and store the value of the `meta.rwgps_datetime` key in the response
3. Upon reception of a webhook for the user, get and integrate the items that have changed from a new sync request at `GET https://ridewithgps.com/api/v1/sync.json?since=[rwgps_datetime]`. Store the new value of `meta.rwgps_datetime` for the next sync request.

Doing a sync request on webhook reception instead of processing the items in the body of the webhook leads to the same result and provides a recovery mechanism for previous webhooks that might have failed because of network or systems availability issues.

### trips

Manage trips

- **`ridewithgps-pp-cli trips delete`** - Deletes a trip owned by the authenticated user
- **`ridewithgps-pp-cli trips get`** - Returns detailed information about a specific trip
- **`ridewithgps-pp-cli trips get-polyline`** - Returns an encoded polyline string for the specified trip.

The polyline is a simplified representation of the trip geometry, suitable for displaying on a map.
- **`ridewithgps-pp-cli trips list`** - Returns a paginated list of trips for the authenticated user.

Supports filtering by `name`, `visibility`, `distance_min`, `distance_max`, `elevation_gain_min`, `elevation_gain_max` and `stationary`.

### users

Manage users

- **`ridewithgps-pp-cli users`** - Returns information about the currently authenticated user


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ridewithgps-pp-cli collections list

# JSON for scripting and agents
ridewithgps-pp-cli collections list --json

# Filter to specific fields
ridewithgps-pp-cli collections list --json --select id,name,status

# Dry run — show the request without sending
ridewithgps-pp-cli collections list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ridewithgps-pp-cli collections list --agent
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
ridewithgps-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ride-gps-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `RIDEWITHGPS_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ridewithgps-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ridewithgps-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $RIDEWITHGPS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Failed to authenticate the request** — Set RIDEWITHGPS_API_KEY and RIDEWITHGPS_AUTH_TOKEN (the v1 API needs both, sent as x-rwgps-* headers), or harvest a token with 'auth-tokens --user-email <e> --user-password <pw>'.
- **List commands return only some of my routes** — The v1 API returns only assets you own; routes shared with you must be copied into your account first.
- **export writes no files** — Run 'sync' first so track_points are mirrored locally, or pass --native to stream Ride with GPS's own file render.
- **Requests slow down or pause** — Rate limits are undocumented; the CLI backs off automatically on 429 and retries — let it finish.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**ridewithgps-mcp**](https://github.com/boezzz/ridewithgps-mcp) — TypeScript (6 stars)
- [**RideWithGPS**](https://github.com/SteveWinward/RideWithGPS) — C# (3 stars)
- [**ridewithgps-client**](https://github.com/jmoseley/ridewithgps-client) — TypeScript (2 stars)
- [**pyrwgps**](https://github.com/ckdake/pyrwgps) — Python (1 stars)
- [**ride-cli**](https://github.com/ridewithgps/ride-cli) — TypeScript (1 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
