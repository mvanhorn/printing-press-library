---
name: pp-slack
description: "Printing Press CLI for Slack. Send messages, search conversations, monitor channels, and manage your Slack workspace from the terminal Capabilities include: activity, analytics, bots, conversations, digest, dnd, emoji, feedback, files, funny, health, messages, pins, profile, quiet, reactions, reminders, response-times, search, stars, tail, team, threads-stale, trends, usergroups, users. Trigger phrases: 'install slack', 'use slack', 'run slack', 'Slack commands', 'setup slack'."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["slack-pp-cli"],"env":["SLACK_BOT_TOKEN"]},"primaryEnv":"SLACK_BOT_TOKEN","install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@latest","bins":["slack-pp-cli"],"label":"Install via go install"}]}}'
---

# Slack — Printing Press CLI

Send messages, search conversations, monitor channels, and manage your Slack workspace from the terminal

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `slack-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@latest
   ```

   If `@latest` installs a stale build (the Go module proxy cache can lag the repo by hours after a fresh merge), install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-cli@main
   ```
3. Verify: `slack-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.
5. Auth setup — set the API key and register it with the CLI:
   ```bash
   export SLACK_BOT_TOKEN="your-key-here"
   slack-pp-cli auth set-token
   ```
   Run `slack-pp-cli doctor` to verify credentials.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-mcp@latest
   ```

   If `@latest` installs a stale build, install from main directly:
   ```bash
   GOPRIVATE='github.com/mvanhorn/*' GOFLAGS=-mod=mod \
     go install github.com/mvanhorn/printing-press-library/library/productivity/slack/cmd/slack-pp-mcp@main
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add -e SLACK_BOT_TOKEN=value slack-pp-mcp -- slack-pp-mcp
   ```
   Ask the user for actual values of required API keys before running.
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which slack-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Discover commands: `slack-pp-cli --help`
   Key commands:
   - `activity` — User activity summary across channels from local sync data
   - `analytics` — Run analytics queries on locally synced data
   - `bots` — Get information about a bot user
   - `conversations` — List all channels in the workspace
   - `digest` — Daily or weekly digest from locally synced activity
   - `dnd` — Get DND status for the authenticated user
   - `emoji` — List all custom emoji for the workspace
   - `feedback` — Record feedback about this CLI (local by default; upstream opt-in)
   - `files` — Get information about a file
   - `funny` — Find the funniest locally synced messages from public channels
   - `health` — Channel health report from locally synced activity
   - `messages` — Get a permalink URL for a message
   - `pins` — List pinned items in a channel
   - `profile` — Named sets of flags saved for reuse
   - `quiet` — Find quiet or dead channels from locally synced data
   - `reactions` — Get reactions for a message
   - `reminders` — List all reminders for the authenticated user
   - `response-times` — Average first-response time in threads from local sync data
   - `search` — Full-text search across synced data or live API
   - `stars` — List starred items
   - `tail` — Stream live changes by polling the API at regular intervals
   - `team` — Get workspace access logs (requires admin)
   - `threads-stale` — Find unanswered or stale threads from local sync data
   - `trends` — Channel activity trends by week from local sync data
   - `usergroups` — List all user groups in the workspace
   - `users` — List all users in the workspace
3. Match the user query to the best command. Drill into subcommand help if needed: `slack-pp-cli <command> --help`
4. Execute with the `--agent` flag:
   ```bash
   slack-pp-cli <command> [subcommand] [args] --agent
   ```
5. The `--agent` flag sets `--json --compact --no-input --no-color --yes` for structured, token-efficient output.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
