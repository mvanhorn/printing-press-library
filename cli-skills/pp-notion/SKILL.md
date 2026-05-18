---
name: pp-notion
description: "Notion CLI — manage pages and blocks, sync workspace to local SQLite, detect stale pages, and track changes. Trigger phrases: `find stale notion pages`, `sync notion to local`, `use notion-pp-cli`, `run notion`, `export notion page`."
author: "Nikica Jokic"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - notion-pp-cli
    install:
      - kind: go
        bins: [notion-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/notion/cmd/notion-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/notion/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Notion — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `notion-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install notion --cli-only
   ```
2. Verify: `notion-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/notion/cmd/notion-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

notion-pp-cli syncs your Notion workspace into a local SQLite store and exposes commands for page and block management, stale page detection, and change tracking. Works offline and ships a full MCP server.

## When to Use This CLI

Use notion-pp-cli when you need to manage Notion pages and blocks from the terminal, detect stale pages, or track what changed since a given time. Run `sync` once to populate the local store, then `stale` and `changed` work offline. Not a substitute for the Notion web UI for day-to-day editing.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`changed`** — Show everything in the workspace added, edited, or deleted since your last sync or a given timestamp.

  _Use at the start of an agent session to orient on what has changed before taking action._

  ```bash
  notion-pp-cli changed --since 2h --json
  ```

### Local state that compounds
- **`stale`** — List pages and records not edited in N days, filterable by database, parent, or tag.

  _Use to identify dead pages before workspace cleanup or to flag deliverables overdue for review._

  ```bash
  notion-pp-cli stale --days 30 --db ProjectDB --agent
  ```

## Command Reference

**blocks** — Block endpoints

- `notion-pp-cli blocks delete` — Delete a block
- `notion-pp-cli blocks query-meeting-notes` — Query meeting notes
- `notion-pp-cli blocks get` — Get a block by ID
- `notion-pp-cli blocks update` — Update a block
- `notion-pp-cli blocks children list` — List children of a block or page
- `notion-pp-cli blocks children append` — Append blocks to a page or block

**comments** — Comment endpoints

- `notion-pp-cli comments create-a` — Create a comment
- `notion-pp-cli comments delete-a` — Delete a comment
- `notion-pp-cli comments list` — List comments
- `notion-pp-cli comments retrieve` — Retrieve a comment
- `notion-pp-cli comments update-a` — Update a comment

**custom-emojis** — Custom emoji endpoints

- `notion-pp-cli custom-emojis` — List custom emojis

**data-sources** — Data source endpoints

- `notion-pp-cli data-sources create-a-database` — Create a data source
- `notion-pp-cli data-sources retrieve-a` — Retrieve a data source
- `notion-pp-cli data-sources update-a` — Update a data source

**databases** — Database endpoints

- `notion-pp-cli databases query <database_id>` — Query records in a database with optional filter/sort
- `notion-pp-cli databases create` — Create a database
- `notion-pp-cli databases retrieve` — Retrieve a database
- `notion-pp-cli databases update` — Update a database

**file-uploads** — File upload endpoints

- `notion-pp-cli file-uploads create-file` — Create a file upload
- `notion-pp-cli file-uploads list` — List file uploads
- `notion-pp-cli file-uploads retrieve` — Retrieve a file upload

**notion-search** — Manage notion search

- `notion-pp-cli notion-search` — Search by title

**oauth** — OAuth endpoints (basic authentication)

- `notion-pp-cli oauth create-a-token` — Exchange an authorization code for an access and refresh token
- `notion-pp-cli oauth introspect-token` — Introspect a token
- `notion-pp-cli oauth revoke-token` — Revoke a token

**pages** — Page endpoints

- `notion-pp-cli pages update` — Update a page
- `notion-pp-cli pages create` — Create a page
- `notion-pp-cli pages get` — Get a page by ID
- `notion-pp-cli pages move` — Move a page
- `notion-pp-cli pages markdown export` — Export a page as markdown

**users** — User endpoints

- `notion-pp-cli users get` — List all users
- `notion-pp-cli users get-self` — Retrieve your token's bot user
- `notion-pp-cli users get-userid` — Retrieve a user

**views** — View endpoints

- `notion-pp-cli views create` — Create a view
- `notion-pp-cli views delete` — Delete a view
- `notion-pp-cli views list` — List views
- `notion-pp-cli views retrieve-a` — Retrieve a view
- `notion-pp-cli views update-a` — Update a view

**sync pages** — Sync all pages and databases to local SQLite

- `notion-pp-cli sync pages` — Sync all pages and databases via search API (run before stale/changed)
- `notion-pp-cli sync pages --full` — Full resync from scratch
- `notion-pp-cli sync pages --type database` — Sync databases only


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
notion-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Morning triage: what changed overnight

```bash
notion-pp-cli changed --since 8h --json
```

Show everything edited in the last 8 hours — use at session start to orient before taking action.

### Find stale pages before cleanup

```bash
notion-pp-cli stale --days 30 --json --select id,title,days_since_edit,last_edited_time
```

Pages untouched for 30+ days, sorted oldest-first. The Notion UI has no equivalent filter across multiple workspaces.

### Export a page as markdown

```bash
notion-pp-cli pages markdown export <page-id>
```

Fetches the page content as markdown via the native Notion markdown endpoint — no conversion needed.

### Read block children of a page

```bash
notion-pp-cli blocks children list <page-id> --json --select id,type
```

List all top-level blocks on a page with their types and IDs.

## Auth Setup

Requires a Notion Internal Integration token. Create one at notion.so/my-integrations, share your top-level pages with it, then set `NOTION_BEARER_AUTH` (or `NOTION_TOKEN`) in your environment.

First-run setup:
```bash
notion-pp-cli doctor          # verify auth
notion-pp-cli sync pages      # sync all pages + databases (~15k pages, takes 2-3 min)
notion-pp-cli stale --days 30 # now works
notion-pp-cli changed --since 24h
```

Run `notion-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  notion-pp-cli comments list --block-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
notion-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
notion-pp-cli feedback --stdin < notes.txt
notion-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.notion-pp-cli/feedback.jsonl`. They are never POSTed unless `NOTION_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `NOTION_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
notion-pp-cli profile save briefing --json
notion-pp-cli --profile briefing comments list --block-id 550e8400-e29b-41d4-a716-446655440000
notion-pp-cli profile list --json
notion-pp-cli profile show briefing
notion-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `notion-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/notion/cmd/notion-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add notion-pp-mcp -- notion-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which notion-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   notion-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `notion-pp-cli <command> --help`.
