# Whoop CLI

Created by [@gregvanhorn](https://github.com/gregvanhorn) (Greg Van Horn).
Contributors: [@bobeglz](https://github.com/bobeglz) (Roberto González Grajeda), [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `whoop-pp-cli` binary and the `pp-whoop` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install whoop
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install whoop --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install whoop --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install whoop --agent claude-code
npx -y @mvanhorn/printing-press-library install whoop --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/whoop/cmd/whoop-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/whoop-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install whoop --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-whoop --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-whoop --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install whoop --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/whoop-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `WHOOP_OAUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "whoop": {
      "command": "whoop-pp-mcp",
      "env": {
        "WHOOP_OAUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Whoop uses OAuth 2.0 — there is no static API key. You first create an OAuth
app at [developer.whoop.com](https://developer.whoop.com), then exchange a
browser-based authorization code for a Bearer token. The CLI automates the
browser flow.

#### One-time: register a redirect URI in the Whoop dashboard

The CLI's `auth login` command starts a tiny local web server that catches the
OAuth callback. By default it listens on **`http://localhost:8085/callback`**,
and Whoop will reject the flow unless that exact URI is pre-registered on your
OAuth app.

1. Go to [developer.whoop.com](https://developer.whoop.com) → your OAuth app.
2. Under **Redirect URIs**, add: `http://localhost:8085/callback`
3. Save.

If you'd rather use a different port, register `http://localhost:<port>/callback`
in the dashboard and pass `--port <port>` on `auth login`.

#### Run the login flow

```bash
whoop-pp-cli auth login \
  --client-id <your-client-id> \
  --client-secret <your-client-secret>
```

The CLI will open your browser, you approve the requested scopes, Whoop
redirects back to `localhost:8085/callback`, and the access + refresh tokens
are stored in the local config. From then on, every command authenticates
automatically.

If you already have a Bearer token, you can also set it directly:

```bash
export WHOOP_OAUTH="your-token-here"
```

#### Troubleshooting: `redirect_uri does not match`

If you see "The OAuth2 request resulted in an error… The 'redirect_uri'
parameter does not match any of the OAuth 2.0 Client's pre-registered
redirect urls", the URI you used at login is not registered on the Whoop
dashboard. Either:

- Register `http://localhost:8085/callback` in the Whoop OAuth app, or
- Pick a port that's already registered, e.g. `auth login --port 9000` if
  `http://localhost:9000/callback` is what you registered.

### 3. Verify Setup

```bash
whoop-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
whoop-pp-cli activity-mapping mock-value
```

## Usage

Run `whoop-pp-cli --help` for the full command reference and flag list.

## Commands

### activity

Manage activity

- **`whoop-pp-cli activity get-sleep-by-id`** - Get the sleep for the specified ID
- **`whoop-pp-cli activity get-sleep-collection`** - Get all sleeps for a user, paginated. Results are sorted by start time in descending order.
- **`whoop-pp-cli activity get-workout-by-id`** - Get the workout for the specified ID
- **`whoop-pp-cli activity get-workout-collection`** - Get all workouts for a user, paginated. Results are sorted by start time in descending order.

### activity-mapping

Manage activity mapping

- **`whoop-pp-cli activity-mapping get`** - Lookup the V2 UUID for a given V1 activity ID

### cycle

Manage cycle

- **`whoop-pp-cli cycle get-by-id`** - Get the cycle for the specified ID
- **`whoop-pp-cli cycle get-collection`** - Get all physiological cycles for a user, paginated. Results are sorted by start time in descending order.

### partner

Endpoints for trusted WHOOP partner operations

- **`whoop-pp-cli partner add-test-data`** - Generates test user and lab requisition data for partner integration testing. This endpoint is only available in non-production environments
- **`whoop-pp-cli partner get-lab-requisition-by-id`** - Retrieves a lab requisition with its associated service requests by its unique identifier. The requesting partner must be an owner of the lab requisition.
- **`whoop-pp-cli partner get-service-request-by-id`** - Retrieves a service request by its unique identifier. The requesting partner must be an owner of the service request.
- **`whoop-pp-cli partner request-token`** - Exchanges partner client credentials for an access token.
- **`whoop-pp-cli partner update-service-request-status`** - Updates the business status of a service request task. The requesting partner must be an owner of the service request.
- **`whoop-pp-cli partner upload-diagnostic-report-results`** - Creates a diagnostic report with results for a service request. The requesting partner must be an owner of the service request.

### recovery

Manage recovery

- **`whoop-pp-cli recovery get-collection`** - Get all recoveries for a user, paginated. Results are sorted by start time of the related sleep in descending order.

### user

Endpoints for retrieving user profile and measurement data.

- **`whoop-pp-cli user get-body-measurement`** - Retrieves the body measurements (height, weight, max heart rate) for the authenticated user.
- **`whoop-pp-cli user get-profile-basic`** - Retrieves the basic profile information (name, email) for the authenticated user.
- **`whoop-pp-cli user revoke-oauth-access`** - Revoke the access token granted by the user. If the associated OAuth client is configured to receive webhooks, it will no longer receive them for this user.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
whoop-pp-cli activity-mapping mock-value

# JSON for scripting and agents
whoop-pp-cli activity-mapping mock-value --json

# Filter to specific fields
whoop-pp-cli activity-mapping mock-value --json --select id,name,status

# Dry run — show the request without sending
whoop-pp-cli activity-mapping mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
whoop-pp-cli activity-mapping mock-value --agent
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
whoop-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/whoop-pp-cli/config.toml`

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `WHOOP_OAUTH` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `whoop-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $WHOOP_OAUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
