---
name: pp-vestaboard
description: "Read, render Trigger phrases: `what's on my vestaboard`, `show my vestaboard`, `send a message to my vestaboard`, `post to the vestaboard`, `use vestaboard`, `run vestaboard`."
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - vestaboard-pp-cli
    install:
      - kind: go
        bins: [vestaboard-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/vestaboard/cmd/vestaboard-pp-cli
---

# Vestaboard — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `vestaboard-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install vestaboard --cli-only
   ```
2. Verify: `vestaboard-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/vestaboard/cmd/vestaboard-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI to read or change what is displayed on a Vestaboard split-flap display: check the current message, render it as text, format text into character codes, post a new message, or adjust the transition style. It is the right tool when an agent needs to observe or update a physical Vestaboard programmatically.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for the Vestaboard Local (LAN) API or the Subscription API — it wraps the Cloud Read/Write API only.
- Do not use it to manage multiple boards or board membership; a Cloud token maps to a single board.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Make the board legible
- **`message preview`** — See what's on your Vestaboard right now as readable text instead of a raw integer grid.

  _Reach for this before composing or sending — it is the only way to know the board's current contents without decoding codes by hand._

  ```bash
  vestaboard-pp-cli message preview --json
  ```
- **`characters`** — Print the full Vestaboard character-code table (code to glyph) for hand-building a character-array message.

  _Use this when constructing a 'message send' character payload so you map glyphs to the right integer codes._

  ```bash
  vestaboard-pp-cli characters --json
  ```

## Command Reference

**message** — The message currently shown on the Vestaboard.

- `vestaboard-pp-cli message get` — Read the message currently displayed on the board.
- `vestaboard-pp-cli message send` — Send a new message to the board.

**transition** — Transition animation settings for the board (Flagship and Note devices).

- `vestaboard-pp-cli transition get` — Get the current transition style and speed.
- `vestaboard-pp-cli transition set` — Set the transition style and speed. Both fields are required.

**vbml** — Vestaboard Markup Language (VBML) text formatting service.

- `vestaboard-pp-cli vbml` — Convert a text string into a 2D array of Vestaboard character codes.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
vestaboard-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Read the board as text

```bash
vestaboard-pp-cli message preview
```

Decodes the live layout into a bordered block of glyphs you can read.

### Inspect the raw layout with field selection

```bash
vestaboard-pp-cli message get --agent --select currentMessage.id,currentMessage.layout
```

Pulls just the id and raw character grid for an agent without the surrounding envelope.

### Format then send a message

```bash
vestaboard-pp-cli vbml --message "GM" --json
```

Convert text to a character-code grid on the VBML host, then pass it to 'message send --body-json'.

### Set a transition

```bash
vestaboard-pp-cli transition set --transition wave --transition-speed gentle
```

Change the flip animation style and speed.

## Auth Setup

Create a Cloud API token in the API tab on web.vestaboard.com (or the mobile app under Settings → Advanced Settings), then run 'vestaboard-pp-cli auth set-token <token>'. The token is sent only to cloud.vestaboard.com; the VBML formatter on vbml.vestaboard.com takes no credentials.

Run `vestaboard-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  vestaboard-pp-cli message get --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
vestaboard-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
vestaboard-pp-cli feedback --stdin < notes.txt
vestaboard-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/vestaboard-pp-cli/feedback.jsonl`. They are never POSTed unless `VESTABOARD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VESTABOARD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
vestaboard-pp-cli profile save briefing --json
vestaboard-pp-cli --profile briefing message get
vestaboard-pp-cli profile list --json
vestaboard-pp-cli profile show briefing
vestaboard-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `vestaboard-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/vestaboard/cmd/vestaboard-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add vestaboard-pp-mcp -- vestaboard-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which vestaboard-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   vestaboard-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `vestaboard-pp-cli <command> --help`.
