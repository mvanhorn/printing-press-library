# TV Time CLI

**Every TV Time feature, plus stats, agenda, next, and backlog queries no TV Time tool has.**

Drives 'what to watch tonight' and 'how am I doing this year' surfaces from the same TV Time account you already use on your phone.

Learn more at [TV Time](https://api2.tozelabs.com).

Created by [@ekrof](https://github.com/ekrof) (Jonas Ekerhovd).

## Install

The recommended path installs both the `tv-time-pp-cli` binary and the `pp-tv-time` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tv-time
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tv-time --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tv-time --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tv-time --agent claude-code
npx -y @mvanhorn/printing-press-library install tv-time --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tv-time/cmd/tv-time-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tv-time-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->

## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tv-time --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tv-time --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tv-time --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tv-time --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tv-time-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TVTIME_USERNAME` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tv-time/cmd/tv-time-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tv-time": {
      "command": "tv-time-pp-mcp",
      "env": {
        "TVTIME_USERNAME": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

TV Time has no public API. This CLI authenticates against the mobile backend (api2.tozelabs.com) with your account username and password over HTTP Basic, then resolves your user id via a signin handshake. Set TVTIME_USERNAME and TVTIME_PASSWORD, then run 'tv-time-pp-cli doctor' to confirm the handshake.

## Quick Start

```bash
# Check the binary can run and env is set
tv-time-pp-cli doctor --dry-run

# Sign in with HTTP Basic and capture your user_id
tv-time-pp-cli session

# Profile rollup — episodes, hours, top shows
tv-time-pp-cli stats "$USER_ID" --json

# Pick the single best next episode across all in-progress shows
tv-time-pp-cli next "$USER_ID"

# Top 10 shows by unwatched-episode count
tv-time-pp-cli backlog "$USER_ID" --limit 10

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Personal analytics
- **`stats`** — Roll up your watched history into episodes, estimated hours, and your most-watched shows — straight from /user/{user_id}/stats.

  _Pipe your watch totals into a dashboard, a homepage stat widget, or a year-in-review post._

  ```bash
  tv-time-pp-cli stats "$USER_ID" --json
  ```

### Calendar
- **`agenda`** — See what airs today or in the next N days for the shows you follow.

  _Drives a daily 'what's new' email, a Raycast widget, or a homepage 'on tonight' block._

  ```bash
  tv-time-pp-cli agenda "$USER_ID" --days 7
  ```

### Queue
- **`next`** — Pick the single best next episode to watch across all your in-progress shows.

  _Removes the choose-what-to-watch tax. Wire it into a 'one button to start tonight's show' flow._

  ```bash
  tv-time-pp-cli next "$USER_ID"
  ```
- **`backlog`** — Rank the shows you follow by how many unwatched episodes are piling up.

  _Tells you which shows are slipping. Useful for 'shows I should drop' or 'shows to catch up on this weekend'._

  ```bash
  tv-time-pp-cli backlog "$USER_ID" --limit 10
  ```
- **`since`** — (stub) Show what changed in your queue since a point in time. Not yet wired — requires a local sync mirror with historical snapshots that isn't implemented yet.

  _Stubbed for now; intent is to ship once the sync mirror lands._

  ```bash
  tv-time-pp-cli since --help
  ```

### Bulk write
- **`binge`** — (stub) Bulk-mark a whole season or show as watched in one command. Not yet wired — requires a season-episode listing from a sync mirror that isn't implemented yet.

  _Stubbed for now so the shape is visible; intent is to ship once the sync mirror lands._

  ```bash
  tv-time-pp-cli binge --help
  ```

## Recipes

### Bulk-log a finished season

```bash
tv-time-pp-cli binge --help  # (stub) — not yet wired
```

Bulk-mark is a stub awaiting the local sync mirror. Use `tv-time-pp-cli episodes mark-watched` per episode in the meantime.

### Tonight's lineup as JSON

```bash
tv-time-pp-cli agenda "$USER_ID" --days 1 --json
```

Filter the calendar payload to just episodes airing in the next day for a compact, agent-friendly result.

### Your TV year

```bash
tv-time-pp-cli stats "$USER_ID" --json
```

Roll up episodes watched, estimated hours, and top shows for your account.

### What to watch right now

```bash
tv-time-pp-cli next "$USER_ID"
```

Get one decisive next episode across all your in-progress shows.

## Usage

Run `tv-time-pp-cli --help` for the full command reference and flag list.

## Commands

### episodes

Your watch queue, history, and episode actions

- **`tv-time-pp-cli episodes comment`** - Comment on an episode
- **`tv-time-pp-cli episodes delete-actor-vote`** - Remove your actor vote on an episode
- **`tv-time-pp-cli episodes delete-comment`** - Delete one of your episode comments
- **`tv-time-pp-cli episodes delete-reaction`** - Remove your reaction from an episode
- **`tv-time-pp-cli episodes mark-unwatched`** - Mark an episode as not watched
- **`tv-time-pp-cli episodes mark-watched`** - Mark an episode as watched
- **`tv-time-pp-cli episodes react`** - React to an episode (star-meter). emotion_id: great=1 wow=3 ok=6 bad=7 good=8
- **`tv-time-pp-cli episodes to-watch`** - Episodes in your to-watch queue
- **`tv-time-pp-cli episodes vote-actor`** - Vote for an actor on an episode
- **`tv-time-pp-cli episodes watched`** - Episodes you have watched (paged, most recent first)

### session

Authenticate against TV Time

- **`tv-time-pp-cli session`** - Sign in (HTTP Basic) and resolve your user id

### shows

Search and browse TV shows

- **`tv-time-pp-cli shows actors`** - List actors for a show
- **`tv-time-pp-cli shows followed`** - Shows you follow
- **`tv-time-pp-cli shows for-later`** - Shows you saved for later
- **`tv-time-pp-cli shows my-shows`** - Your shows grouped by category (watching, up_to_date, finished, for_later, ...)
- **`tv-time-pp-cli shows search`** - Search shows by name

### user

Your profile, stats, friends, calendar, and notifications

- **`tv-time-pp-cli user badges`** - Your earned badges
- **`tv-time-pp-cli user calendar`** - Calendar of upcoming airings for shows you follow
- **`tv-time-pp-cli user friends`** - Your friends list
- **`tv-time-pp-cli user notifications`** - Your notifications
- **`tv-time-pp-cli user profile`** - Your profile
- **`tv-time-pp-cli user stats`** - Your viewing stats

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tv-time-pp-cli shows search --q example-value

# JSON for scripting and agents
tv-time-pp-cli shows search --q example-value --json

# Filter to specific fields
tv-time-pp-cli shows search --q example-value --json --select id,name,status

# Dry run — show the request without sending
tv-time-pp-cli shows search --q example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tv-time-pp-cli shows search --q example-value --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `TV_TIME_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
tv-time-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/tv-time-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name              | Kind     | Required | Description                 |
| ----------------- | -------- | -------- | --------------------------- |
| `TVTIME_USERNAME` | per_call | Yes      | Set to your API credential. |
| `TVTIME_PASSWORD` | per_call | Yes      | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tv-time-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting

**Authentication errors (exit code 4)**

- Run `tv-time-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TVTIME_USERNAME`
  **Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 from any user-scoped command** — Re-run `tv-time-pp-cli session` to refresh the user_id; TVTIME_USERNAME / TVTIME_PASSWORD must be set.
- **`binge` or `since` returns '(stub) not yet wired'** — These commands are stubs awaiting the local sync mirror. Use `episodes mark-watched` per episode in the meantime.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tvtimewrapper**](https://github.com/seanwlk/tvtimewrapper) — Python
- [**tvtime-api**](https://github.com/EdgarVaguencia/tvtime-api) — JavaScript
- [**tvtime-client**](https://github.com/srgsf/tvtime-client) — Java
- [**tvtime-api-notes**](https://github.com/TheIndra55/tvtime-api) — Markdown

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
