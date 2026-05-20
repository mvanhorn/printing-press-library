---
name: pp-excalidraw-mcp
description: "Printing Press CLI for Excalidraw Mcp. Combined CLI for multiple API services"
author: "bk20260126-code"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - excalidraw-mcp-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/developer-tools/excalidraw-mcp/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Excalidraw Mcp — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `excalidraw-mcp-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install excalidraw-mcp --cli-only
   ```
2. Verify: `excalidraw-mcp-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Combined CLI for multiple API services

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`diff`** — Compare two canvas snapshots and see exactly which elements were added, removed, or moved.

  _Use when you need to audit diagram changes, review what an AI agent drew, or verify a refactoring didn't break diagram structure._

  ```bash
  excalidraw-mcp-pp-cli diff v1 v2 --json
  ```
- **`stats`** — See element type distribution, color palette in use, and bounding box summary for the current canvas.

  _Use to understand canvas composition before asking an AI to modify or extend a diagram._

  ```bash
  excalidraw-mcp-pp-cli stats --json --agent
  ```

### Agent-native plumbing
- **`convert`** — Convert a Mermaid diagram file to a PNG or SVG in one command — no separate steps needed.

  _Use in CI/CD pipelines to turn Mermaid specs into diagram images without manual canvas interaction._

  ```bash
  excalidraw-mcp-pp-cli convert --input flow.mmd --output diagram.png
  ```
- **`stale`** — Walk a directory for .excalidraw files and flag diagrams that haven't been updated in N days.

  _Use in CI to catch documentation diagrams that may be out of date with the codebase they describe._

  ```bash
  excalidraw-mcp-pp-cli stale --dir ./docs --since 90d --json
  ```
- **`agent-canvas-context`** — Emit a compact canvas summary (element count, type histogram, bounding box) sized for agent context windows.

  _Use at the start of any agent task that involves the canvas so the agent knows current state without reading all elements._

  ```bash
  excalidraw-mcp-pp-cli agent-canvas-context --agent --compact
  ```

## Command Reference

**collections** — Manage collections

- `excalidraw-mcp-pp-cli collections cloud-create` — Create a scene collection
- `excalidraw-mcp-pp-cli collections cloud-delete` — Delete a collection
- `excalidraw-mcp-pp-cli collections cloud-get` — Get collection metadata
- `excalidraw-mcp-pp-cli collections cloud-list` — List scene collections
- `excalidraw-mcp-pp-cli collections cloud-update` — Update collection metadata

**elements** — Manage elements

- `excalidraw-mcp-pp-cli elements batch` — Add an array of elements to the canvas in a single request. Preserves IDs when provided.
- `excalidraw-mcp-pp-cli elements clear` — Remove every element from the canvas. Irreversible unless a snapshot was saved first.
- `excalidraw-mcp-pp-cli elements create` — Add a new shape, text, or arrow to the Excalidraw canvas.
- `excalidraw-mcp-pp-cli elements delete` — Delete a canvas element
- `excalidraw-mcp-pp-cli elements from-mermaid` — Parse Mermaid diagram syntax and place the resulting elements on the canvas.
- `excalidraw-mcp-pp-cli elements get` — Get element by ID
- `excalidraw-mcp-pp-cli elements list` — Returns every element currently on the Excalidraw canvas.
- `excalidraw-mcp-pp-cli elements search` — Filter canvas elements by type, position, or bounding box coordinates.
- `excalidraw-mcp-pp-cli elements update` — Modify any property of an existing canvas element.

**excalidraw-canvas-cloud-export** — Manage excalidraw canvas cloud export

- `excalidraw-mcp-pp-cli excalidraw-canvas-cloud-export` — Render the current canvas to a PNG or SVG file. Requires the browser canvas to be open. Returns base64-encoded image...

**excalidraw-canvas-cloud-health** — Manage excalidraw canvas cloud health

- `excalidraw-mcp-pp-cli excalidraw-canvas-cloud-health` — Check if the canvas server is running and return element count and WebSocket status.

**excalidraw-canvas-cloud-sync** — Manage excalidraw canvas cloud sync

- `excalidraw-mcp-pp-cli excalidraw-canvas-cloud-sync` — Canvas sync status and memory usage

**files** — Manage files

- `excalidraw-mcp-pp-cli files delete` — Delete an image file
- `excalidraw-mcp-pp-cli files list` — List image files on the canvas
- `excalidraw-mcp-pp-cli files upload` — Upload image files to the canvas

**invites** — Manage invites

- `excalidraw-mcp-pp-cli invites cloud-create` — Send a workspace invitation
- `excalidraw-mcp-pp-cli invites cloud-delete` — Cancel an invitation
- `excalidraw-mcp-pp-cli invites cloud-list` — List pending workspace invitations

**logs** — Manage logs

- `excalidraw-mcp-pp-cli logs` — Retrieve workspace audit logs

**scenes** — Manage scenes

- `excalidraw-mcp-pp-cli scenes cloud-create` — Create a cloud scene
- `excalidraw-mcp-pp-cli scenes cloud-delete` — Delete a cloud scene
- `excalidraw-mcp-pp-cli scenes cloud-get` — Get cloud scene metadata
- `excalidraw-mcp-pp-cli scenes cloud-list` — List all scenes in your Excalidraw Plus workspace.
- `excalidraw-mcp-pp-cli scenes cloud-update` — Update cloud scene metadata

**snapshots** — Manage snapshots

- `excalidraw-mcp-pp-cli snapshots create` — Capture the current canvas state. Use before destructive operations or to mark versions.
- `excalidraw-mcp-pp-cli snapshots get` — Get snapshot by name
- `excalidraw-mcp-pp-cli snapshots list` — Returns all saved canvas checkpoints with name, element count, and creation time.

**viewport** — Manage viewport

- `excalidraw-mcp-pp-cli viewport` — Adjust the visible area: zoom level, pan position, or auto-fit all elements.

**workspace** — Manage workspace

- `excalidraw-mcp-pp-cli workspace cloud-get` — Get workspace metadata
- `excalidraw-mcp-pp-cli workspace cloud-list-users` — List workspace members
- `excalidraw-mcp-pp-cli workspace cloud-remove-user` — Remove a member from the workspace


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
excalidraw-mcp-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `excalidraw-mcp-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
excalidraw-mcp-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `EXCALIDRAW_API_KEY` as an environment variable.

Run `excalidraw-mcp-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  excalidraw-mcp-pp-cli elements list --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
excalidraw-mcp-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
excalidraw-mcp-pp-cli feedback --stdin < notes.txt
excalidraw-mcp-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.excalidraw-mcp-pp-cli/feedback.jsonl`. They are never POSTed unless `EXCALIDRAW_MCP_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `EXCALIDRAW_MCP_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
excalidraw-mcp-pp-cli profile save briefing --json
excalidraw-mcp-pp-cli --profile briefing elements list
excalidraw-mcp-pp-cli profile list --json
excalidraw-mcp-pp-cli profile show briefing
excalidraw-mcp-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `excalidraw-mcp-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add excalidraw-mcp-pp-mcp -- excalidraw-mcp-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which excalidraw-mcp-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   excalidraw-mcp-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `excalidraw-mcp-pp-cli <command> --help`.
