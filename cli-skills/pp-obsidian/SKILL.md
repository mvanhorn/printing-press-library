---
name: pp-obsidian
description: "Read-only Obsidian vault analytics CLI. Wraps the official `obsidian` binary (v1.12+) for live reads and maintains a local SQLite mirror for offline compound analytics (health score, orphans with age ranking, stale notes, broken wikilinks with source context, raw SQL). V1 is read-only by design — writes wait on the upstream markdown-patch frontmatter-corruption fix."
author: "LARGE FORMAT"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - obsidian-pp-cli
    install:
      - kind: go
        bins: [obsidian-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/obsidian/cmd/obsidian-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/obsidian/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Obsidian — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `obsidian-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install obsidian --cli-only
   ```
2. Verify: `obsidian-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/obsidian/cmd/obsidian-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**deadends** — Manage deadends

- `obsidian-pp-cli deadends` — List notes with no outgoing links

**files** — Manage files

- `obsidian-pp-cli files` — Wraps `obsidian files`. Filter by folder or extension.

**folders** — Manage folders

- `obsidian-pp-cli folders` — Wraps `obsidian folders`.

**live-search** — Manage live search

- `obsidian-pp-cli live-search context` — Wraps `obsidian search:context query=<text>` — body-text search with surrounding lines for each hit.
- `obsidian-pp-cli live-search live-search` — Wraps `obsidian search query=<text>` — dials the running Obsidian process for body-text search. Distinct from the...

**notes** — Single-note read operations

- `obsidian-pp-cli notes <name>` — Wraps `obsidian read file={name}` (or path= for exact paths). Returns raw file contents.

**tags** — Manage tags

- `obsidian-pp-cli tags` — Wraps `obsidian tags`. Optionally scope to a file via path= query.

**tasks** — Manage tasks

- `obsidian-pp-cli tasks` — Wraps `obsidian tasks`. Filter by done/todo/status. (task mutation is V2.)

**unresolved** — Manage unresolved

- `obsidian-pp-cli unresolved` — Wraps `obsidian unresolved`. Tier-3 `broken` builds on this with source-note context.

**vault** — Vault-level inventory and metadata

- `obsidian-pp-cli vault` — Wraps `obsidian vault`. info= narrows to one field.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
obsidian-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `obsidian-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  obsidian-pp-cli deadends --agent --select id,name,status
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
obsidian-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
obsidian-pp-cli feedback --stdin < notes.txt
obsidian-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.obsidian-pp-cli/feedback.jsonl`. They are never POSTed unless `OBSIDIAN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OBSIDIAN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
obsidian-pp-cli profile save briefing --json
obsidian-pp-cli --profile briefing deadends
obsidian-pp-cli profile list --json
obsidian-pp-cli profile show briefing
obsidian-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `obsidian-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/obsidian/cmd/obsidian-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add obsidian-pp-mcp -- obsidian-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which obsidian-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   obsidian-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `obsidian-pp-cli <command> --help`.
