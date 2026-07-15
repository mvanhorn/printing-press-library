# TickTick CLI

**Every TickTick surface — tasks, habits, focus, daily notes — with a corruption-proof daily-note contract and offline analytics no other TickTick tool has.**

Covers both TickTick APIs: the official Open API and the richer internal V2 surface (habits, focus records, completed-task history, batch ops). The 'note edit' command encodes a field-whitelist write contract that cannot corrupt TEXT-kind daily notes, and the local SQLite mirror powers 'agenda', 'review', 'habits streaks', and 'focus stats' — ritual-shaped commands that replace multi-call fan-outs.

## Install

The recommended path installs both the `ticktick-pp-cli` binary and the `pp-ticktick` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ticktick
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ticktick --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ticktick --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ticktick --agent claude-code
npx -y @mvanhorn/printing-press-library install ticktick --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ticktick/cmd/ticktick-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticktick-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ticktick --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ticktick --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ticktick --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ticktick --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticktick-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TICKTICK_SESSION_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ticktick/cmd/ticktick-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ticktick": {
      "command": "ticktick-pp-mcp",
      "env": {
        "TICKTICK_SESSION_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Two tiers. The V2 internal API (habits, focus, daily notes, batch) authenticates with a session token: set TICKTICK_USERNAME and TICKTICK_PASSWORD and the CLI signs on automatically, or set TICKTICK_SESSION_TOKEN with the 't' cookie from a logged-in browser session (required for Google-SSO accounts without a password). The optional V1 Open API tier uses TICKTICK_API_TOKEN (OAuth bearer from developer.ticktick.com). Requests send a browser-like User-Agent because the API returns 503 to default clients.

## Quick Start

```bash
# Health check — verifies config shape and which auth tiers are configured, no credentials needed
ticktick-pp-cli doctor --dry-run

# Hydrate the local SQLite mirror that powers agenda, review, and streaks
ticktick-pp-cli sync --resources tasks,habits,focus

# Today's tasks, habits with checkin state, and focus sessions in one bounded response
ticktick-pp-cli agenda --json

# Preview a safe daily-note append — the contract that can never flip the note's kind
ticktick-pp-cli note edit --date today --append "14:30 deep-work block done" --dry-run

# Week-in-review data pack, narrowed with --select to keep agent context small
ticktick-pp-cli review --since 7d --json --select completed.title,focus_totals

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Corruption-proof writes
- **`note edit`** — Edit your daily note's content with a corruption-proof contract — the command can never flip the note's task kind or break its subtasks.

  _Use this instead of a raw task update whenever the target is a daily note; raw updates through generic tools have corrupted notes before._

  ```bash
  ticktick-pp-cli note edit --date today --append "20:15 wrapped the printing-press build" --dry-run
  ```

### Local state that compounds
- **`agenda`** — One command returns today's tasks, habits with checkin state, and focus sessions together.

  _Reach for this at session start instead of fanning out list calls; one bounded response covers the whole briefing._

  ```bash
  ticktick-pp-cli agenda --json --select tasks.title,habits.name
  ```
- **`review`** — Gather a week of completed tasks, daily notes, focus totals, and habit checkins as one structured pack ready for synthesis.

  _Use for weekly-review synthesis instead of paging completed tasks and re-reading seven daily notes by hand._

  ```bash
  ticktick-pp-cli review --since 7d --json
  ```
- **`habits streaks`** — Current streak, longest streak, and at-risk-today flags for every habit.

  _Answers 'which habits are about to break' without hand-rolling date math over raw checkins._

  ```bash
  ticktick-pp-cli habits streaks --json
  ```
- **`focus stats`** — Focus/pomodoro time aggregated per day or per project over any window.

  _Use for weekly focus totals instead of scraping raw focus records._

  ```bash
  ticktick-pp-cli focus stats --since 7d --json
  ```

## Recipes

### Morning briefing

```bash
ticktick-pp-cli agenda --json --agent
```

One bounded response with today's tasks, habit state, and focus sessions for a session-start briefing.

### Log a focus block to the daily note

```bash
ticktick-pp-cli note edit --date today --append "15:00-15:45 client audit prep"
```

Timestamped append through the corruption-proof write path.

### Friday review pack, narrowed for agents

```bash
ticktick-pp-cli review --since 7d --agent --select completed.title,notes.content,focus_totals.by_day
```

Deeply nested week pack narrowed with --select so synthesis doesn't drown in raw JSON.

### Which habits are about to break

```bash
ticktick-pp-cli habits streaks --json --select name,current_streak,at_risk
```

Streak math over synced checkins with only the decision-relevant fields.

### Weekly focus hours by project

```bash
ticktick-pp-cli focus stats --since 7d --by project --json
```

Aggregated pomodoro/focus durations from the local mirror.

## Usage

Run `ticktick-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TICKTICK_CONFIG_DIR`, `TICKTICK_DATA_DIR`, `TICKTICK_STATE_DIR`, or `TICKTICK_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TICKTICK_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TICKTICK_HOME=/srv/ticktick
ticktick-pp-cli doctor
```

Under `TICKTICK_HOME=/srv/ticktick`, the four dirs resolve to `/srv/ticktick/config`, `/srv/ticktick/data`, `/srv/ticktick/state`, and `/srv/ticktick/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ticktick": {
      "command": "ticktick-pp-mcp",
      "env": {
        "TICKTICK_HOME": "/srv/ticktick"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TICKTICK_DATA_DIR` overrides an explicit `--home` for that kind. Use `TICKTICK_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TICKTICK_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `ticktick-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### completed

Completed task history

- **`ticktick-pp-cli completed`** - Completed tasks in a date window

### focus

Focus / pomodoro records

- **`ticktick-pp-cli focus`** - Focus and pomodoro records, most recent first

### habits

Habits and habit checkins

- **`ticktick-pp-cli habits checkin`** - Upsert habit checkins (batch add/update)
- **`ticktick-pp-cli habits checkins`** - Query habit checkins after a date stamp
- **`ticktick-pp-cli habits list`** - List all habits

### projects

Projects (lists)

- **`ticktick-pp-cli projects`** - List all projects

### tags

Tags

- **`ticktick-pp-cli tags`** - List all tags

### tasks

Tasks — all uncompleted tasks via the V2 sync surface, single-task get, and batch mutations

- **`ticktick-pp-cli tasks batch`** - Batch create/update/delete tasks (arrays of task objects; updates must carry id, projectId, etag and must NOT include kind)
- **`ticktick-pp-cli tasks get`** - Get a single task by id (full JSON incl. kind, etag, childIds)
- **`ticktick-pp-cli tasks list`** - List all uncompleted tasks (from the V2 sync check)

### user

Account profile

- **`ticktick-pp-cli user`** - Authenticated user profile


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ticktick-pp-cli completed

# JSON for scripting and agents
ticktick-pp-cli completed --json

# Filter to specific fields
ticktick-pp-cli completed --json --select id,name,status

# Dry run — show the request without sending
ticktick-pp-cli completed --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ticktick-pp-cli completed --agent
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
ticktick-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `ticktick-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ticktick-pp-cli/config.toml`; `--home`, `TICKTICK_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TICKTICK_SESSION_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `ticktick-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ticktick-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TICKTICK_SESSION_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 503 on every request** — The API rejects default HTTP clients; the CLI sends a browser-like User-Agent automatically — if you overrode it in config, remove the override.
- **V2 signon fails but the password is correct (Google-SSO account)** — Google-SSO accounts may have no password. Set one at ticktick.com account settings, or copy the 't' cookie from DevTools into TICKTICK_SESSION_TOKEN.
- **note edit reports an etag conflict** — The note changed since the last read; re-run the command — it re-fetches the task and retries once automatically.
- **agenda or review returns empty** — Run 'ticktick-pp-cli sync --resources tasks,habits,focus' first; these commands read the local mirror.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tick-mcp**](https://github.com/kpihx/tick-mcp) — Python
- [**ticktick-mcp-server**](https://github.com/liadgez/ticktick-mcp-server) — TypeScript
- [**ticktick-py**](https://github.com/lazeroffmichael/ticktick-py) — Python
- [**ticktick-api-v2**](https://github.com/OliverStoll/ticktick-api-v2) — Python
- [**ticktick-sdk**](https://github.com/dev-mirzabicer/ticktick-sdk) — Python
- [**ticktick-mcp**](https://github.com/jacepark12/ticktick-mcp) — Python
- [**mcp-ticktick**](https://github.com/karbassi/mcp-ticktick) — TypeScript
- [**ticktick-cli**](https://github.com/avilabss/ticktick-cli) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
