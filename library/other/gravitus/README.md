# Gravitus CLI

**The only CLI that syncs your Gravitus strength data into your training dashboard.**

Gravitus has no API and no data export. gravitus-pp-cli handles the session auth, paginates your full workout history, and writes LiftingSession records directly into your dashboard's SQLite database — incremental, reliable, and scriptable.

Learn more at [Gravitus](https://gravitus.com).

Created by [@azaaron](https://github.com/azaaron) (mvanhorn).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `gravitus-pp-cli` binary and the `pp-gravitus` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install gravitus
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install gravitus --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install gravitus --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install gravitus --agent claude-code
npx -y @mvanhorn/printing-press-library install gravitus --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/gravitus/cmd/gravitus-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gravitus-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install gravitus --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-gravitus --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-gravitus --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install gravitus --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
gravitus-pp-cli auth login-password
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/gravitus-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "gravitus": {
      "command": "gravitus-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Gravitus uses Django session auth. Run `gravitus-pp-cli auth login-password` with your email and password — the CLI handles the CSRF token exchange and stores your session cookie in the config file. Re-run `auth login` whenever the session expires (typically every few weeks).

## Quick Start

```bash
# authenticate — handles the CSRF dance automatically
gravitus-pp-cli auth login

# pull all workouts into dev.db as LiftingSession records
gravitus-pp-cli gravitus-sync --dashboard-db ./prisma/dev.db

# only fetch new workouts since last sync
gravitus-pp-cli gravitus-sync --incremental --dashboard-db ./prisma/dev.db

# view all personal records as structured JSON
gravitus-pp-cli exercises prs --agent

# find lifts with no progress in 6 weeks
gravitus-pp-cli exercises plateau --weeks 6

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Dashboard pipeline
- **`sync`** — Sync all Gravitus workouts into your training dashboard's SQLite database — writes LiftingSession records in the exact Prisma schema format with auth, pagination, and incremental support.

  _Use to populate the training dashboard's lifting data — the only reliable way to authenticate and paginate all workout history into dev.db._

  ```bash
  gravitus-pp-cli gravitus-sync --dashboard-db ./prisma/dev.db
  ```
- **`export`** — Export your complete Gravitus training history to CSV or JSON — the first and only way to get your data out of Gravitus.

  _Use when a coach, analyst, or AI agent needs the full training history outside the app._

  ```bash
  gravitus-pp-cli export --format csv --output training_history.csv
  ```

### Analytics
- **`exercises plateau`** — Identifies exercises where estimated 1RM hasn't improved in N weeks — alert-style output for the dashboard coaching panel.

  _Use before a program change — gives evidence-based list of which lifts need intervention._

  ```bash
  gravitus-pp-cli exercises plateau --weeks 6 --agent
  ```
- **`stats volume`** — Weekly total lifting volume (lbs) aggregated from all synced sessions — the same metric the dashboard LiftingSection displays.

  _Use to feed the dashboard's volume trend chart or check load progression over a training block._

  ```bash
  gravitus-pp-cli stats volume --weeks 12 --agent
  ```
- **`exercises prs`** — All-time PRs across every exercise, extracted from PR markers on workout pages.

  _Use to display the personal records panel in the dashboard or track PR cadence._

  ```bash
  gravitus-pp-cli exercises prs --agent
  ```

## Usage

Run `gravitus-pp-cli --help` for the full command reference and flag list.

## Commands

### accounts

Authentication — login and session management

- **`gravitus-pp-cli accounts`** - Fetch login page to retrieve CSRF token

### exercises

Exercise history, personal records, and volume trends

- **`gravitus-pp-cli exercises <exercise_slug>`** - Fetch exercise history with PR timeline and volume data

### users

User profile and paginated workout history

- **`gravitus-pp-cli users <user_id>`** - Fetch user profile and paginated workout history list

### workouts

Workout sessions with exercises, sets, reps, weight, and PRs

- **`gravitus-pp-cli workouts <workout_id>`** - Fetch full workout detail — exercises, sets, reps, weight, personal records

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
gravitus-pp-cli exercises mock-value

# JSON for scripting and agents
gravitus-pp-cli exercises mock-value --json

# Filter to specific fields
gravitus-pp-cli exercises mock-value --json --select id,name,status

# Dry run — show the request without sending
gravitus-pp-cli exercises mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
gravitus-pp-cli exercises mock-value --agent
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

## Health Check

```bash
gravitus-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/gravitus-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GRAVITUS_SESSION_ID` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `gravitus-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GRAVITUS_SESSION_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **sync returns 0 workouts** — run `gravitus-pp-cli doctor` — session likely expired, re-run `auth login`
- **dashboard-db write fails** — verify the path with `gravitus-pp-cli doctor --dashboard-db <path>` and ensure the file is not locked by the Next.js dev server
- **auth login fails with CSRF error** — run `gravitus-pp-cli auth logout` then `auth login` again to force a fresh CSRF token fetch

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hevycli**](https://github.com/obay/hevycli) — Go
- [**hevy-mcp**](https://github.com/chrisdoc/hevy-mcp) — TypeScript
- [**LiftShift**](https://github.com/aree6/LiftShift) — JavaScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
