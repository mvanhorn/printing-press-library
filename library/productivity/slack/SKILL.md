---
name: pp-slack
description: "Slack workspace CLI for the terminal. Send messages, search channels and DMs, list conversations, get user/bot/emoji info, analyze channel health, find stale threads, and sync the workspace locally for fast offline queries. Two auth surfaces coexist: SLACK_BOT_TOKEN (xoxb-, for workspace-wide read + post) and SLACK_USER_TOKEN (xoxp-, for user-scoped actions like DM history or search). Use when the user asks to send a Slack message, search Slack, check channel activity, summarize a digest, find who's on a team, find stale threads, analyze channel health, list users / emoji / reminders / pinned items, or wants offline-capable Slack queries."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - slack-pp-cli
      env:
        - SLACK_BOT_TOKEN
    primaryEnv: SLACK_BOT_TOKEN
    install:
      - kind: go
        bins: [slack-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli
---

# Slack - Printing Press CLI

Send messages, search conversations, monitor channels, and manage your Slack workspace from the terminal. Pairs live-API calls with a local SQLite sync so read-heavy analytics (activity trends, stale threads, channel health) run offline without burning your rate limit.

## When to Use This CLI

Reach for this when the user wants:

- send a Slack message to a channel or DM
- search Slack across channels, DMs, and threads (live or synced)
- list users, channels, usergroups, custom emoji, pinned items, or reminders
- understand channel health (quiet channels, activity trends, response times, digest)
- find stale or unanswered threads across the workspace
- pull an activity summary for a user or channel
- export channel history to JSONL for archival or migration
- discover the funniest messages in public channels (local-sync-powered novelty query)

Skip it when the user wants to create Slack apps, workflows, or manage admin-only team settings beyond `team` access logs. Those surfaces are better served by the Slack web admin.

## Two Auth Surfaces

Slack has two parallel token types and this CLI supports both:

| Token | Scopes | When to use |
|-------|--------|-------------|
| `SLACK_BOT_TOKEN` (xoxb-) | workspace-scoped bot permissions | Default for post-message, read-channel, list-users, etc. |
| `SLACK_USER_TOKEN` (xoxp-) | user-scoped | DM history, search on behalf of a user, stars, reminders |

Set whichever the workspace permits. If both are set, user-token wins for user-scoped endpoints and bot-token otherwise. Get a token at `https://api.slack.com/apps` -> your app -> OAuth & Permissions.

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** -> show `slack-pp-cli --help`
2. **Starts with `install`** -> ends with `mcp` -> MCP installation; otherwise -> CLI installation
3. **Anything else** -> Direct Use (map to the best command and run it)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+).
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@latest
   ```
3. Verify: `slack-pp-cli --version`.
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup:
   ```bash
   export SLACK_BOT_TOKEN="xoxb-..."
   # optional, enables user-scoped endpoints (DM history, search-as-user):
   export SLACK_USER_TOKEN="xoxp-..."
   ```
6. Verify: `slack-pp-cli doctor` reports token status and workspace identity.

## MCP Server Installation

The CLI also ships an MCP server at `slack-pp-mcp`. Install and register:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-mcp@latest
claude mcp add -e SLACK_BOT_TOKEN=xoxb-... slack-pp-mcp -- slack-pp-mcp
```

Ask the user for the actual token value before running.

## Direct Use

1. Check installed: `which slack-pp-cli`. If missing, offer CLI installation.
2. Discover commands: `slack-pp-cli --help`; drill into `slack-pp-cli <cmd> --help`.
3. Match the user query to the best command (see Notable Commands below).
4. Execute with `--agent` for structured output:
   ```bash
   slack-pp-cli <command> [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes`.

Source routing (local vs live) is controlled by `--data-source`: `auto` (default) prefers local-synced data with live fallback; `live` always hits the API; `local` runs fully offline against the SQLite snapshot. Run `slack-pp-cli sync` first to populate the local store for analytics queries.

## Notable Commands

| Command | What it does |
|---------|--------------|
| `conversations` | List channels and DMs in the workspace |
| `users` | List all users in the workspace |
| `search <query>` | Full-text search across synced messages (or live API with `--data-source live`) |
| `digest` | Daily/weekly activity digest from locally synced data |
| `health` | Channel health report (activity, engagement, stagnation) |
| `quiet` | Find dead or low-activity channels |
| `response-times` | Average first-response time in threads |
| `threads-stale` | Unanswered or stale threads |
| `activity <user-or-channel>` | Activity summary across channels from local sync |
| `trends` | Week-over-week channel activity trends |
| `sync` | Populate the local SQLite store for offline analytics |
| `emoji` / `reminders` / `pins` / `stars` | Workspace directory queries |
| `team` | Access logs (requires admin token) |

Run any command with `--help` for full flag documentation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields, with dotted-path support (see below)
- **Previewable** — `--dry-run` shows the request without sending
- **Cacheable** — GET responses cached for 5 minutes, bypass with `--no-cache`
- **Non-interactive** — never prompts, every input is a flag


### Filtering output

`--select` accepts dotted paths to descend into nested responses; arrays traverse element-wise:

```bash
slack-pp-cli <command> --agent --select id,name
slack-pp-cli <command> --agent --select items.id,items.owner.name
```

Use this to narrow huge payloads to the fields you actually need — critical for deeply nested API responses.


### Response envelope

Data-layer commands wrap output in `{"meta": {...}, "results": <data>}`. Parse `.results` for data and `.meta.source` to know whether it's `live` or local. The `N results (live)` summary is printed to stderr only when stdout is a TTY; piped/agent consumers see pure JSON on stdout.


## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (channel, user, message) |
| 4 | Authentication required (token missing or invalid) |
| 5 | API error (Slack upstream, including `not_in_channel`, `channel_not_found`) |
| 7 | Rate limited (Slack 429; CLI honors `Retry-After`) |
