---
name: pp-braze
description: "Printing Press CLI for Braze. Read-first Braze REST API surface for marketing analytics, campaign inspection, content assets, and catalog lookup."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - braze-pp-cli
    install:
      - kind: go
        bins: [braze-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/braze/cmd/braze-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/marketing/braze/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Braze — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `braze-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install braze --cli-only
   ```
2. Verify: `braze-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/braze/cmd/braze-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-first Braze REST API surface for marketing analytics, campaign inspection, content assets, and catalog lookup.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**campaigns** — Manage campaigns

- `braze-pp-cli campaigns get-data-series` — Export campaign analytics over time.
- `braze-pp-cli campaigns get-details` — Export metadata and configuration for a campaign.
- `braze-pp-cli campaigns list` — Export a list of Braze campaigns.

**canvas** — Manage canvas

- `braze-pp-cli canvas get-data-series` — Export Canvas analytics over time.
- `braze-pp-cli canvas get-data-summary` — Export summarized Canvas analytics.
- `braze-pp-cli canvas get-details` — Export metadata and configuration for a Canvas.
- `braze-pp-cli canvas list-canvases` — Export a list of Braze Canvases.

**catalogs** — Manage catalogs

- `braze-pp-cli catalogs` — List catalogs.

**content-blocks** — Manage content blocks

- `braze-pp-cli content-blocks get-info` — Get content block details.
- `braze-pp-cli content-blocks list` — List content blocks.

**custom-attributes** — Manage custom attributes

- `braze-pp-cli custom-attributes` — Export custom attributes and metadata.

**events** — Manage events

- `braze-pp-cli events get-data-series` — Export custom event analytics over time.
- `braze-pp-cli events list-custom` — Export custom event definitions and metadata.
- `braze-pp-cli events list-custom-names` — Export names of custom events recorded for an app group.

**kpi** — Manage kpi

- `braze-pp-cli kpi get-daily-active-users` — Export daily active user counts.
- `braze-pp-cli kpi get-monthly-active-users` — Export monthly active user counts.
- `braze-pp-cli kpi get-new-users` — Export daily new user counts.

**preference-center** — Manage preference center

- `braze-pp-cli preference-center get` — Get preference center details.
- `braze-pp-cli preference-center list` — List preference centers.

**segments** — Manage segments

- `braze-pp-cli segments get-data-series` — Export estimated segment size over time.
- `braze-pp-cli segments get-details` — Export metadata for a segment.
- `braze-pp-cli segments list` — Export a list of Braze segments.

**sessions** — Manage sessions

- `braze-pp-cli sessions` — Export app session counts over time.

**templates** — Manage templates

- `braze-pp-cli templates get-email-info` — Get email template details.
- `braze-pp-cli templates list-email` — List email templates.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
braze-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `braze-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
braze-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `BRAZE_BEARER_AUTH` as an environment variable.

Run `braze-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  braze-pp-cli campaigns list --agent --select id,name,status
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
braze-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
braze-pp-cli feedback --stdin < notes.txt
braze-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/github.com/mvanhorn/printing-press-library/library/marketing/braze/feedback.jsonl`. They are never POSTed unless `BRAZE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BRAZE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
braze-pp-cli profile save briefing --json
braze-pp-cli --profile briefing campaigns list
braze-pp-cli profile list --json
braze-pp-cli profile show briefing
braze-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `braze-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/braze/cmd/braze-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add braze-pp-mcp -- braze-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which braze-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   braze-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `braze-pp-cli <command> --help`.
