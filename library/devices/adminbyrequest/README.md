# Admin By Request CLI

**Every Admin By Request portal action, plus a local SQLite mirror of audit, events, inventory and requests for ad-hoc SQL and offline search.**

Pull audit log, events, inventory and requests into a local store with one sync, then approve or deny elevations, generate offline PIN codes, and answer cross-resource questions (repeat requestors, agent-version drift, audit/event correlation) the portal does not surface.

Created by [@joltsconsulting](https://github.com/joltsconsulting) (joltsconsulting).

## Install

The recommended path installs both the `adminbyrequest-pp-cli` binary and the `pp-adminbyrequest` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install adminbyrequest
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install adminbyrequest --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install adminbyrequest --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install adminbyrequest --agent claude-code
npx -y @mvanhorn/printing-press-library install adminbyrequest --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/cmd/adminbyrequest-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/adminbyrequest-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install adminbyrequest --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-adminbyrequest --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-adminbyrequest --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install adminbyrequest --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/adminbyrequest-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `ADMINBYREQUEST_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "adminbyrequest": {
      "command": "adminbyrequest-pp-mcp",
      "env": {
        "ADMINBYREQUEST_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set ADMINBYREQUEST_API_KEY in your environment. The CLI sends it via the apikey header. AbR enforces a 100,000-call daily quota per tenant; the CLI tracks calls locally and warns you before exhaustion via quota forecast.

## Quick Start

```bash
# Confirm key works and detect which AbR data center your tenant lives on.
adminbyrequest-pp-cli doctor

# Pull audit log, events, inventory and requests into the local SQLite store.
adminbyrequest-pp-cli sync --full

# Surface pending elevation requests to approve or deny.
adminbyrequest-pp-cli requests list --status pending --json

# Identify users repeatedly requesting elevation over the last 30 days.
adminbyrequest-pp-cli requests repeat-offenders --window 30d --json

# Check whether the current API usage pace will exceed the 100k daily quota.
adminbyrequest-pp-cli quota forecast --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`requests repeat-offenders`** — Surface the top users by elevation-request count over a configurable window, so admins can spot patterns the portal does not visualize.

  _When tracing repeated elevation attempts across a fleet, this is the single query that names the people; without it, an agent has to walk every request and aggregate manually._

  ```bash
  adminbyrequest-pp-cli requests repeat-offenders --window 30d --json
  ```
- **`correlate`** — Given an audit log entry ID, join it to nearby events on the same computer to reconstruct a full elevation timeline.

  _Incident response often asks what else happened on this machine when this admin session opened; this is the single command that answers it._

  ```bash
  adminbyrequest-pp-cli correlate 50461167 --window 5m --json
  ```

### Fleet hygiene
- **`inventory drift`** — List endpoints whose AbR client version is older than a target version, useful for upgrade campaigns.

  _Before any compliance audit, the agent should know which endpoints are running an old client; this is the one-liner._

  ```bash
  adminbyrequest-pp-cli inventory drift --client-version 8.7.2 --json
  ```
- **`inventory risk-score`** — Score each endpoint by elevation frequency, local-admin count, and AbR client version recency.

  _Lets an admin focus remediation effort on the endpoints most likely to be abused._

  ```bash
  adminbyrequest-pp-cli inventory risk-score --top 10 --json
  ```

### Reachability mitigation
- **`quota forecast`** — Track local API call count against the 100k-per-day quota and predict whether the current pace will exceed it before midnight.

  _Reachability mitigation: an agent that calls the API in a loop needs to know before it bricks the tenant for the day._

  ```bash
  adminbyrequest-pp-cli quota forecast --json
  ```

### Compliance and review
- **`requests denied-reasons`** — Tokenize the free-text deniedReason field across all denied requests and emit a top-N word distribution.

  _For compliance review and tone-of-policy checks, surfacing the actual words used is faster than reading each row._

  ```bash
  adminbyrequest-pp-cli requests denied-reasons --top 20 --json
  ```
- **`report compliance`** — Render audit entries for a user or computer over a window in a format suitable for auditors (CSV or markdown).

  _Auditor evidence requests always want a single artifact; this generates it in one command._

  ```bash
  adminbyrequest-pp-cli report compliance --since 2026-01-01 --user CHRISCOOMBES --format md
  ```

## Usage

Run `adminbyrequest-pp-cli --help` for the full command reference and flag list.

## Commands

### auditlog

Admin session and elevation audit log entries

- **`adminbyrequest-pp-cli auditlog delta`** - Delta query for audit log changes (use timeNow/deltaTime ticks for resumable sync)
- **`adminbyrequest-pp-cli auditlog list`** - List audit log entries (admin sessions, app elevations, denied requests). Pagination is by id cursor.

### events

Security and operational events emitted by AbR clients

- **`adminbyrequest-pp-cli events list`** - List events. Use code filter to narrow to specific event types.

### inventory

Endpoint inventory: hardware, software, OS, AbR agent version

- **`adminbyrequest-pp-cli inventory list`** - List inventoried endpoints. Use wantsoftware/wanthardware to expand.
- **`adminbyrequest-pp-cli inventory pin`** - Generate an offline elevation PIN code for a device. Pass --challenge for challenge-response mode.

### requests

Pending, approved, and denied elevation requests

- **`adminbyrequest-pp-cli requests approve`** - Approve a pending elevation request.
- **`adminbyrequest-pp-cli requests deny`** - Deny a pending elevation request.
- **`adminbyrequest-pp-cli requests list`** - List elevation requests filtered by status.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
adminbyrequest-pp-cli auditlog list

# JSON for scripting and agents
adminbyrequest-pp-cli auditlog list --json

# Filter to specific fields
adminbyrequest-pp-cli auditlog list --json --select id,name,status

# Dry run — show the request without sending
adminbyrequest-pp-cli auditlog list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
adminbyrequest-pp-cli auditlog list --agent
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
adminbyrequest-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/adminbyrequest-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `ADMINBYREQUEST_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `adminbyrequest-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $ADMINBYREQUEST_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **doctor reports HTTP 500 from every data center** — API key may be wrong; verify ADMINBYREQUEST_API_KEY is the value from your portal Settings > Tenant Settings > Data > API Keys.
- **Sync stalls at exactly 100,000 calls** — You hit the daily quota; AbR blocks the tenant until next business day. Run quota forecast next time to predict before it happens.
- **PIN code endpoint returns General error** — Pass the challenge code via --challenge; the bare endpoint without a challenge returns this error by design.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
