---
name: pp-cmux
description: "Every cmux feature, plus persisted state, cross-surface search, and a notify-driven event stream no other cmux tool... Trigger phrases: `search across cmux panes`, `which cmux workspace is awaiting input`, `show stuck cmux panes`, `watch cmux notifications`, `use cmux`, `run cmux-pp-cli`."
author: "Damien Stevens"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cmux-pp-cli
---

# cmux — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cmux-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cmux --cli-only
   ```
2. Verify: `cmux-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

This CLI wraps the cmux Unix-socket CLI with an AI-agent-shaped surface: a local SQLite store of workspaces, surfaces, status entries, and notifications; FTS5 search across titles and sampled pane content with surface-level matches; and a long-running `watch` command that emits cmux notification events as JSONL to stdout, files, Slack, or any webhook so ecosystem-manager-style agents can wait on events instead of polling.

## When to Use This CLI

Use cmux-pp-cli when an agent or human needs the state across many cmux workspaces in one query, when search must return the surface (not just the workspace) that matched, or when a manager-style loop wants to be notified by event rather than poll every pane. Read-only commands are safe to run while cmux is in active use; write-side actions (focus, send) are gated by explicit flags.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Cross-pane intelligence
- **`search`** — Search workspace titles, surface titles, notification bodies, and sampled pane content with FTS5 — get the surface and snippet, not just the workspace; --switch jumps cmux to the matching surface.

  _When an agent or user knows a phrase but not which workspace, this returns the exact surface to focus instead of forcing a workspace-only walk._

  ```bash
  cmux-pp-cli search 'WAF cookie' --json --select results.workspace_ref,results.surface_ref,results.snippet
  ```
- **`workspaces card`** — One-shot summary for a workspace: metadata (cwd, git_branch, pr) + current status entries + last 3 notifications + last sampled pane snippets per surface.

  _Gives a manager subagent the full picture of a workspace in one tool call._

  ```bash
  cmux-pp-cli workspaces card Tuck --json
  ```

### Notify-driven loop closure
- **`watch`** — Long-running stream of cmux notification events as JSONL, with pluggable sinks (stdout, file, exec hook, Slack webhook, generic webhook). Replaces capture-pane polling with id-cursored notification events or fsnotify on the session JSON.

  _Lets a manager subagent wait on events instead of burning context on full pane scans._

  ```bash
  cmux-pp-cli watch --source notifications --sink stdout --json
  ```
- **`alert add`** — Declarative rules that fire when a workspace transitions into a state (e.g. Tuck claude_code Running -> Needs input). Sinks: stdout, file, exec, macOS osascript, Slack webhook, generic webhook.

  _Closes the loop on the user instead of on cmux's sidebar._

  ```bash
  cmux-pp-cli alert add --workspace Tuck --on awaiting --sink slack:https://hooks.slack.com/services/X --json
  ```

### Local state that compounds
- **`status timeline`** — Time-series of agent state per workspace (and key) over a window, drawn from local status snapshots persisted on every sync.

  _Answers 'when did Tuck go awaiting?' which is needed to triage stuck panes and to bound sync cadence._

  ```bash
  cmux-pp-cli status timeline --workspace Tuck --since 4h --json
  ```
- **`status stuck`** — List every (workspace, key) whose latest persisted state is awaiting input and whose transition timestamp is older than a threshold.

  _Surfaces the panes a manager subagent should triage first, without re-capturing screens._

  ```bash
  cmux-pp-cli status stuck --over 30m --json
  ```
- **`status awaiting`** — Single canonical state per workspace (idle / working / awaiting / stranded) computed by joining status entries with title-icon decode and surface health.

  _Gives an agent a single boolean to drive 'should I look here?' instead of decoding folklore icons._

  ```bash
  cmux-pp-cli status awaiting --all --json
  ```
- **`status changes`** — List workspaces (and keys) whose persisted state changed within a time window, with prev_value and new_value.

  _Direct replacement for the manager's 'did anything change since my last tick?' question, without rewalking every pane._

  ```bash
  cmux-pp-cli status changes --since 1h --json
  ```

## Command Reference

**buffers** — cmux paste buffers

- `cmux-pp-cli buffers` — List paste buffers.

**capabilities** — Methods the running cmux exposes

- `cmux-pp-cli capabilities` — List every RPC method the running cmux exposes.

**hooks** — cmux event hooks

- `cmux-pp-cli hooks` — List configured hooks.

**logs** — Sidebar log entries per workspace

- `cmux-pp-cli logs` — List sidebar log entries for a workspace.

**notifications** — cmux notifications (the event stream for loop closure)

- `cmux-pp-cli notifications` — List notifications across all workspaces. Use --json for the full payload, or filter on workspace_id.

**panes** — cmux panes (split containers inside a workspace)

- `cmux-pp-cli panes` — List panes in a workspace.

**status** — Per-workspace agent status entries (e.g., claude_code state)

- `cmux-pp-cli status` — List status entries for a workspace.

**surfaces** — cmux surfaces (terminal or browser tabs inside a pane)

- `cmux-pp-cli surfaces health` — Surface health: which surfaces are stranded (not in any window).
- `cmux-pp-cli surfaces list` — List surfaces, optionally scoped to a workspace.

**windows** — cmux windows (top-level OS windows)

- `cmux-pp-cli windows current` — Show the current window.
- `cmux-pp-cli windows list` — List all cmux windows.

**workspaces** — cmux workspaces (logical pane groups)

- `cmux-pp-cli workspaces current` — Show the currently selected workspace.
- `cmux-pp-cli workspaces get` — Get a single workspace with sidebar metadata (cwd, branch, pr).
- `cmux-pp-cli workspaces list` — List all workspaces.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cmux-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Bulk awaiting triage

```bash
cmux-pp-cli status awaiting --all --json --select state,workspace_ref,workspace_title,latest_value,changed_at
```

List every workspace with a normalized `awaiting` state, with the last transition time so a manager can pick the oldest one to look at first.

### Stuck > 30 minutes

```bash
cmux-pp-cli status stuck --over 30m --json
```

Returns each (workspace, key) whose latest persisted state is awaiting input older than 30 minutes — the exact list ecosystem-manager wants for triage.

### Find the surface mentioning a phrase, jump to it

```bash
cmux-pp-cli search 'cookie' --switch
```

FTS over titles + notifications + sampled pane content; with `--switch`, calls cmux `surface.focus` on the top match so you land directly on the right tab.

### Replace polling with a notify-driven loop

```bash
cmux-pp-cli watch --source notifications --sink 'exec:/usr/local/bin/handle-cmux.sh' --json
```

Streams every new cmux notification (id-cursored) and pipes the JSON envelope to the user's handler — the closed-loop replacement for capture-pane polling.

### Time-series of a single workspace

```bash
cmux-pp-cli status timeline --workspace Tuck --since 4h --json --select transitioned_at,key,prev_value,new_value
```

Drills into a workspace's recent transitions to answer 'when did this start awaiting?' without re-reading the screen.

## Auth Setup

cmux-pp-cli does not carry credentials itself — it shells out to the cmux binary,
which authenticates against a local Unix socket at `/tmp/cmux.sock` with a
password resolved in this order: `--password` flag, `CMUX_SOCKET_PASSWORD`
env var, then cmux's keychain entry. If `cmux-pp-cli doctor` reports an auth
failure, set one of those and re-run.

Run `cmux-pp-cli doctor` to verify cmux is installed, reachable, and the
socket answers ping.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cmux-pp-cli buffers --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
cmux-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cmux-pp-cli feedback --stdin < notes.txt
cmux-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cmux-pp-cli/feedback.jsonl`. They are never POSTed unless `CMUX_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CMUX_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
cmux-pp-cli profile save briefing --json
cmux-pp-cli --profile briefing buffers
cmux-pp-cli profile list --json
cmux-pp-cli profile show briefing
cmux-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `cmux-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cmux-pp-mcp -- cmux-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cmux-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cmux-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cmux-pp-cli <command> --help`.
