---
name: pp-orgo
description: "Thin Go-binary alias of the Orgo MCP server — every MCP tool, accessible from a shell. Trigger phrases: `orgo cli`, `spin up an orgo desktop`, `screenshot the orgo computer`, `run bash on orgo`, `use orgo from the shell`, `use orgo`."
author: "NickVasilescu"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - orgo-pp-cli
---

# Orgo — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `orgo-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install orgo --cli-only
   ```
2. Verify: `orgo-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Mirrors the @orgo-ai/mcp server's tools as Cobra commands across projects (workspaces), computers, screen actions, shell, and files. The CLI uses `projects` as the resource name to match the Orgo API path; the MCP exposes the same resource as `workspaces` semantically. Use this CLI to script Orgo from cron, CI, or the terminal when the MCP transport is the wrong shape.

## When to Use This CLI

Use this CLI when scripting Orgo workflows in shell, cron, CI, or agent harnesses where the MCP transport is the wrong shape. Same surface as @orgo-ai/mcp; pick whichever fits the calling context.

## Command Reference

**Workspaces (projects)** — Organize computers into named workspaces

- `orgo-pp-cli projects list` — List all workspaces for the authenticated user.
- `orgo-pp-cli projects get <id>` — Return a workspace by ID, including its computers.
- `orgo-pp-cli projects get-by-name <name>` — Look up a workspace by name when you only have the name from config.
- `orgo-pp-cli projects create --name <name>` — Create a new workspace (names must be unique per user).
- `orgo-pp-cli projects delete <id> --yes` — Delete a workspace and all its computers. Cannot be undone.

**Computers (lifecycle)** — Provision and manage virtual computers

- `orgo-pp-cli computers get <id>` — Return computer details including status.
- `orgo-pp-cli computers create --workspace-id <ws> --name <n> [--ram <gb> --cpu <n>]` — Provision a new computer.
- `orgo-pp-cli computers delete <id> --yes` — Permanently delete a computer.
- `orgo-pp-cli computers clone computer <id>` — Clone a computer with the same disk state.
- `orgo-pp-cli computers move computer <id> --project-id <ws>` — Move a computer between workspaces.
- `orgo-pp-cli computers resize computer <id> [--vcpus N --mem-gb N --disk-size-gb N]` — Live-resize a running computer.
- `orgo-pp-cli computers restart computer <id>` — Restart (stop + start).
- `orgo-pp-cli computers ensure-running ensure-computer-running <id>` — Idempotently resume a suspended VM.

**Screen actions** — Drive the desktop

- `orgo-pp-cli computers screenshot get <id>` — Capture a screenshot. Returns base64 PNG or URL.
- `orgo-pp-cli computers click mouse <id> --x N --y N [--button left|right --double]` — Click at coordinates.
- `orgo-pp-cli computers drag mouse <id> --start-x N --start-y N --end-x N --end-y N` — Drag.
- `orgo-pp-cli computers scroll scroll <id> --direction up|down --amount N` — Scroll the mouse wheel.
- `orgo-pp-cli computers type text <id> --text "..."` — Type literal text at the cursor.
- `orgo-pp-cli computers key press <id> --key "Enter"` — Press a key or combination (e.g. `ctrl+c`).
- `orgo-pp-cli computers wait wait <id> --duration <seconds>` — Pause between actions.

**Shell & code execution**

- `orgo-pp-cli computers bash execute <id> --command "<bash>"` — Run a bash command.
- `orgo-pp-cli computers exec execute-python <id> --code "<py>"` — Run Python code. Use `--stdin` for multi-line.

**Files** — Upload and download

- `orgo-pp-cli files list --project-id <ws>` — List files in a workspace (optionally filter by `--desktop-id`).
- `orgo-pp-cli files upload --project-id <ws> --file <path>` — Upload a file (max 10MB).
- `orgo-pp-cli files download --id <file>` — Get a signed download URL (expires in 1h).
- `orgo-pp-cli files export --desktop-id <cmp> --path <path>` — Export a file from inside the VM.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
orgo-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Move a computer between workspaces

```bash
orgo-pp-cli computers move computer cmp_456 --project-id ws_789
```

Reparents a computer to a different project without copying disk state.

### Type into a focused window

```bash
orgo-pp-cli computers type text cmp_456 --text "hello world"
```

Types literal text at the current cursor position; pair with `key press` for control keys like Enter.

### Look up a project by name

```bash
orgo-pp-cli projects get-by-name production
```

When you have a workspace name from configuration but no ID handy.

## Auth Setup

Bearer auth via ORGO_API_KEY (sk_live_...). Get a key at https://www.orgo.ai/workspaces. Same env var the MCP reads.

Run `orgo-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  orgo-pp-cli computers get mock-value --agent --select id,name,status
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
orgo-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
orgo-pp-cli feedback --stdin < notes.txt
orgo-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.orgo-pp-cli/feedback.jsonl`. They are never POSTed unless `ORGO_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ORGO_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
orgo-pp-cli profile save briefing --json
orgo-pp-cli --profile briefing computers get mock-value
orgo-pp-cli profile list --json
orgo-pp-cli profile show briefing
orgo-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `orgo-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add orgo-pp-mcp -- orgo-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which orgo-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   orgo-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `orgo-pp-cli <command> --help`.
