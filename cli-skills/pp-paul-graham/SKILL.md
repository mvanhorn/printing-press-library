---
name: pp-paul-graham
description: "Printing Press CLI for Paul Graham. Static public essay index for paulgraham.com."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - paul-graham-pp-cli
    install:
      - kind: go
        bins: [paul-graham-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/cmd/paul-graham-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/paul-graham/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Paul Graham — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `paul-graham-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install paul-graham --cli-only
   ```
2. Verify: `paul-graham-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/cmd/paul-graham-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Static public essay index for paulgraham.com.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**latest** — List the newest essays from the live index

- `paul-graham-pp-cli latest --limit 10` — Return the newest essays from paulgraham.com/articles.html.

**list** — Browse the essay index

- `paul-graham-pp-cli list --query startup --limit 20` — List essays, optionally filtering by title or slug.

**search** — Search essays

- `paul-graham-pp-cli search` — Search essay titles and slugs; pass the query as the positional argument, for example `startup`.
- `paul-graham-pp-cli search --full-text` — Fetch essay pages and search full text; pass the query as the positional argument.

**read** — Read one essay

- `paul-graham-pp-cli read --max-chars 2000` — Extract readable text for an essay; pass a slug, URL, title, or title substring as the positional argument.

**links** — Extract essay links

- `paul-graham-pp-cli links` — Return links found on an essay page; pass a slug, URL, title, or title substring as the positional argument.

**random** — Pick an essay

- `paul-graham-pp-cli random --seed 42` — Pick a random essay from the live index.

**articles-html** — Manage articles html

- `paul-graham-pp-cli articles-html` — Returns the static HTML essay index from paulgraham.com.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
paul-graham-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

No authentication required.

Run `paul-graham-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  paul-graham-pp-cli list --agent --select title,url
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
paul-graham-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
paul-graham-pp-cli feedback --stdin < notes.txt
paul-graham-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/paul-graham-pp-cli/feedback.jsonl`. They are never POSTed unless `PAUL_GRAHAM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PAUL_GRAHAM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
paul-graham-pp-cli profile save briefing --json
paul-graham-pp-cli --profile briefing latest
paul-graham-pp-cli profile list --json
paul-graham-pp-cli profile show briefing
paul-graham-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `paul-graham-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/cmd/paul-graham-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add paul-graham-pp-mcp -- paul-graham-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which paul-graham-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   paul-graham-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `paul-graham-pp-cli <command> --help`.
