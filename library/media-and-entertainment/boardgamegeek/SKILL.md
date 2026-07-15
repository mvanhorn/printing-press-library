---
name: pp-boardgamegeek
description: "Printing Press CLI for Boardgamegeek. BoardGameGeek's official XMLAPI2 (https://boardgamegeek."
author: "Ryan Cooper"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - boardgamegeek-pp-cli
    install:
      - kind: go
        bins: [boardgamegeek-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/boardgamegeek/cmd/boardgamegeek-pp-cli
---

# Boardgamegeek — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `boardgamegeek-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install boardgamegeek --cli-only
   ```
2. Verify: `boardgamegeek-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/boardgamegeek/cmd/boardgamegeek-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

BoardGameGeek's official XMLAPI2 (https://boardgamegeek.com/xmlapi2), the
canonical database for board games, RPGs, and video games. Every endpoint
returns XML; the generated client normalizes it to JSON so --json/--select
and MCP tools work like any JSON API.

Authorization: as of 2025-07-02 BGG requires a registered application and an
Authorization: Bearer <token> header on every request (register at
https://boardgamegeek.com/applications, then create a token). Requests go to
boardgamegeek.com (no leading www).

Scope: read-only lookup across the BGG taste graph — search, full game/thing
detail with stats and rankings, the live "hot" list, user profiles, user game
collections, logged plays, families, and guilds. No write/mutating endpoints.

Quirk worth knowing: collection (and occasionally thing/plays) can answer with
HTTP 202 and a "your request is queued, retry shortly" body the first time a
large query is requested; repeat the call until the data is returned.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**collection** — A user's owned/rated/wishlisted game collection

- `boardgamegeek-pp-cli collection` — A user's collection by login name. Filter with the 0/1 status flags (own, rated, played, wishlist, want, trade).

**family** — Manage family

- `boardgamegeek-pp-cli family` — A BGG family (a shared series, theme, or IP grouping related things) by id. Comma-separate ids for a batch lookup.

**guild** — Guild details and membership

- `boardgamegeek-pp-cli guild` — A BGG guild by id. Set `members=1` to include the member roster (paginated with `page`).

**hot** — The current BoardGameGeek "Hot" rankings

- `boardgamegeek-pp-cli hot` — The live BoardGameGeek Hot list. `type` selects the ranking (boardgame is the default and most common).

**plays** — Logged play sessions for a user or a game

- `boardgamegeek-pp-cli plays` — Logged play sessions. Provide `username` (a user's plays) or `id` (a game's plays).

**searches** — Manage searches

- `boardgamegeek-pp-cli searches` — Full-text search across the BGG database. Returns matching things (id, name, year).

**thing** — Full detail for one or more things (games, expansions, accessories)

- `boardgamegeek-pp-cli thing` — Full record for a game/expansion/accessory by id (comma-separate ids for a batch).

**user** — Public user profiles, buddies, and guild membership

- `boardgamegeek-pp-cli user` — Public profile for a BGG user by login name.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
boardgamegeek-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `boardgamegeek-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
boardgamegeek-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `BGG_TOKEN` as an environment variable.

Run `boardgamegeek-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  boardgamegeek-pp-cli collection --username example-resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
boardgamegeek-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
boardgamegeek-pp-cli feedback --stdin < notes.txt
boardgamegeek-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/boardgamegeek-pp-cli/feedback.jsonl`. They are never POSTed unless `BOARDGAMEGEEK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BOARDGAMEGEEK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
boardgamegeek-pp-cli profile save briefing --json
boardgamegeek-pp-cli --profile briefing collection --username example-resource
boardgamegeek-pp-cli profile list --json
boardgamegeek-pp-cli profile show briefing
boardgamegeek-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `boardgamegeek-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/boardgamegeek/cmd/boardgamegeek-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add boardgamegeek-pp-mcp -- boardgamegeek-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which boardgamegeek-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   boardgamegeek-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `boardgamegeek-pp-cli <command> --help`.
