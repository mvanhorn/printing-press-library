---
name: pp-metacritic
description: "Printing Press CLI for Metacritic. Undocumented internal JSON API powering metacritic.com (a Fandom property)."
author: "Ryan Cooper"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - metacritic-pp-cli
    install:
      - kind: go
        bins: [metacritic-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/metacritic/cmd/metacritic-pp-cli
---

# Metacritic — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `metacritic-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install metacritic --cli-only
   ```
2. Verify: `metacritic-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/metacritic/cmd/metacritic-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Undocumented internal JSON API powering metacritic.com (a Fandom property).
Reverse-engineered from the site's public web client. Every request carries a
shared public `apiKey` that the site embeds in its own JavaScript bundle, so no
user credentials are required for read access.

One backend (`backend.metacritic.com`) serves every media type. The `mcoTypeId`
query parameter selects the medium on the browse endpoint: 1 = TV shows,
2 = movies, 13 = games. The `{mediaType}` path segment (`games`, `movies`,
`shows`) selects the medium on the title-detail, filters, and review endpoints.

The non-obvious insight: Metacritic is not just a review aggregator, it is a
cross-media taste graph. Every gap between a title's Metascore (critics) and
user score is a signal about critic-audience divergence.

Scope: this spec covers games, movies, and TV shows plus search, filters, title
detail, and critic/user reviews. Music (albums) is served through a separate
query shape and is intentionally left as a follow-up.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**composer** — Manage composer

- `metacritic-pp-cli composer <slug>` — Returns the full composed detail payload for a title, including Metascore, user score, summary, release data

**finder** — Manage finder

- `metacritic-pp-cli finder browse-titles` — Paginated, sortable list of titles for one medium.
- `metacritic-pp-cli finder list-filters` — Returns the filter facets (genres, platforms, streaming networks, product types) valid for the given medium.
- `metacritic-pp-cli finder search-titles` — Full-text search spanning games, movies, TV, and people.

**reviews** — Critic and user reviews for a title

- `metacritic-pp-cli reviews list-critic` — List critic reviews for a title
- `metacritic-pp-cli reviews list-user` — List user reviews for a title


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
metacritic-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `metacritic-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export METACRITIC_API_KEY="<your-key>"
```

Or persist it in `~/.config/metacritic-pp-cli/config.toml`.

Run `metacritic-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  metacritic-pp-cli composer mock-value --media-type games --api-key your-token-here --agent --select id,name,status
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
metacritic-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
metacritic-pp-cli feedback --stdin < notes.txt
metacritic-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/metacritic-pp-cli/feedback.jsonl`. They are never POSTed unless `METACRITIC_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `METACRITIC_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
metacritic-pp-cli profile save briefing --json
metacritic-pp-cli --profile briefing composer mock-value --media-type games --api-key your-token-here
metacritic-pp-cli profile list --json
metacritic-pp-cli profile show briefing
metacritic-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `metacritic-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/metacritic/cmd/metacritic-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add metacritic-pp-mcp -- metacritic-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which metacritic-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   metacritic-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `metacritic-pp-cli <command> --help`.
