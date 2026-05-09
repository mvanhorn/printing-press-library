# Peloton CLI

Peloton workout, ride, and music history API. No public spec — reverse-engineered from the members.onepeloton.com Auth0 SPA. Endpoint paths and response shapes can shift unannounced; the auth bearer token is harvested from Auth0 SPA localStorage rather than a documented OAuth flow.

Learn more at [Peloton](https://api.onepeloton.com).

Printed by [@twidtwid](https://github.com/twidtwid) (Todd Dailey).

## Install

The recommended path installs both the `peloton-pp-cli` binary and the `pp-peloton` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install peloton
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install peloton --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peloton-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-peloton --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-peloton --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-peloton skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-peloton. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Sign in (Auth0 SPA browser harvest)

Peloton publishes no token portal — every endpoint sits behind a bearer token issued by Auth0's SPA on `members.onepeloton.com`. The fastest path is `auth login`, which spawns Chrome via chromedp, lets you sign in interactively (handles 2FA / captcha / "verify it's you"), then extracts the token from `localStorage` and saves it to `~/.config/peloton-pp-cli/config.toml`:

```bash
peloton-pp-cli auth login
```

Optionally pre-fill the form. **Pass these inline (single-command scope), don't `export` them** — the generated CLI tries `PELOTON_USERNAME` as a bearer token if it stays in the env:

```bash
PELOTON_USERNAME='you@onepeloton.com' PELOTON_PASSWORD='…' peloton-pp-cli auth login
```

The Chrome profile persists at `~/.config/peloton-pp-cli/chrome/`, so subsequent logins reuse session cookies and finish in seconds (form prefill isn't even needed once cookies are set). Tokens last about an hour Peloton-side; re-run `auth login` when they expire.

**Manual fallback:** if you'd rather not use the browser harvest, sign in to `members.onepeloton.com` in your own browser, open DevTools → Network, copy the `Authorization: Bearer …` value off any API request, then paste it:

```bash
peloton-pp-cli auth set-token <paste-token-here>
```

### 3. Verify Setup

```bash
peloton-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
peloton-pp-cli workouts list mock-value
```

## Usage

Run `peloton-pp-cli --help` for the full command reference and flag list.

## Commands

### identity

Authenticated user identity

- **`peloton-pp-cli identity me`** - Get the authenticated user's identity (id, username, profile fields). Used implicitly by every workout query to resolve the user_id.

### rides

Ride metadata and playlists

- **`peloton-pp-cli rides details`** - Get a ride's metadata and playlist (song order, artists, in-class liked-flag, start-time offsets). The workout's ride_id from `workouts list` is the input here. Some on-demand rides ship with empty playlists (instructor talk-only) — that's a normal `playlist.songs: []`, not an error.

### workouts

Workout history (list, show)

- **`peloton-pp-cli workouts get`** - Get a single workout by id, with the same ride/instructor join surface as `list`.
- **`peloton-pp-cli workouts list`** - List the authenticated user's workouts, newest-first. Uses joins=ride,ride.instructor to pull the ride title, instructor name, and ride id alongside each workout.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
peloton-pp-cli workouts list mock-value

# JSON for scripting and agents
peloton-pp-cli workouts list mock-value --json

# Filter to specific fields
peloton-pp-cli workouts list mock-value --json --select id,name,status

# Dry run — show the request without sending
peloton-pp-cli workouts list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
peloton-pp-cli workouts list mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-peloton -g
```

Then invoke `/pp-peloton <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-mcp@latest
```

Then register it:

```bash
claude mcp add peloton peloton-pp-mcp -e PELOTON_USERNAME=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/peloton-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PELOTON_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "peloton": {
      "command": "peloton-pp-mcp",
      "env": {
        "PELOTON_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
peloton-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/peloton-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PELOTON_USERNAME` | auth_flow_input | No | Peloton account email or username consumed by `auth login` to pre-fill the Auth0 sign-in form. Optional — you can sign in by hand inside the spawned Chrome window. |
| `PELOTON_PASSWORD` | auth_flow_input | No | Set during initial auth setup. |
| `PELOTON_TOKEN` | harvested | No | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `peloton-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PELOTON_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
