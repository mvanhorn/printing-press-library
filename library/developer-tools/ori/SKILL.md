---
name: pp-ori
description: "Unified ops CLI for the openclaw A2A bridge: chat, tasks, approvals, service kickstart, and FTS5 search across... Trigger phrases: `send a message to ori`, `what is ori working on right now`, `search past ori tasks for`, `kickstart the openclaw bridge`, `check the openclaw stack health`, `list pending approvals across agents`, `resume the wedged ori task`, `use ori`, `run ori`."
author: "error"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ori-pp-cli
---

# Ori — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ori-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ori --cli-only
   ```
2. Verify: `ori-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/ori/cmd/ori-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Replaces the per-agent MCP tool sprawl (chat_ori, chat_adam, resume_ori, ...) with one cohesive CLI that has a local SQLite mirror, FTS5 search, and a doctor/kickstart pair that bundles the recurring openclaw-stack diagnostic and recovery rituals. Targets a single home-lab operator; loopback only.

## When to Use This CLI

Reach for ori when you want shell-level access to the openclaw A2A agents without going through Claude Code's MCP tools, when you need to answer historical questions about agent activity (search, contexts, watch), or when something is wrong with the bridge and you want one verb that diagnoses or repairs it.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Operational rescue
- **`doctor`** — Run every openclaw stack health check in one verb: launchd a2a-server status, /healthz reachability, both agents responding, agents.toml valid, plugin cache env vars commented in compose, codex OAuth fresh, gateway WS tunnel reachable.

  _When ori chat returns empty failed, this is the one verb that finds the cause across launchd, plugin cache, OAuth, and gateway state._

  ```bash
  ori doctor --json
  ```
- **`kickstart`** — Restart the launchd a2a-server bridge and poll /healthz until ready. Wraps the post-compose-down/up recovery ritual into one command.

  _After every NAS compose down/up the bridge needs this dance or chat returns empty failed; replaces a memorized 2-step incantation._

  ```bash
  ori kickstart --wait
  ```

### Local state that compounds
- **`sync`** — Paginate ListTasks across all configured agents, upsert into the local SQLite mirror, refresh approvals. Foundation for offline search and historical queries.

  _Run this before any tasks search or contexts list to ensure the local store reflects current bridge state._

  ```bash
  ori sync --full
  ```
- **`tasks search`** — Full-text search across cached agent response text. Filters: --agent, --since, --state. Returns task_id, agent, state, first matching line.

  _Answers 'what did I have ori work on related to X' — a question the bridge cannot answer at all._

  ```bash
  ori tasks search 'kanban hygiene' --since 7d --agent ori
  ```
- **`contexts list`** — Group cached tasks by context_id and surface conversations: first task, last task, task count, peek of the first user message.

  _Resume a forgotten thread by context rather than memorizing a task_id from earlier._

  ```bash
  ori contexts list --agent ori --since 24h
  ```

### Operational visibility
- **`watch`** — Poll ListTasks at an interval and print state transitions live: 'ori task abc123: running → input_required at 23:01'.

  _Check whether ori is still working without paying for a Claude Code turn._

  ```bash
  ori watch --agent ori --interval 5s
  ```
- **`logs tail`** — Tail ~/Library/Logs/openclaw-a2a-server/{stdout,stderr}.log via --stream switching. Hides the path.

  _Surface a2a-server stderr instantly during a wedged-bridge incident; no path memorization._

  ```bash
  ori logs tail --stream stderr --lines 100
  ```
- **`approvals pending`** — Aggregate approvals across all agents, with --watch for live polling. Single pane replacing two separate MCP bridge tools.

  _See and decide approvals from both ori and adam in one stream rather than alternating bridge tools._

  ```bash
  ori approvals pending --watch
  ```

## Command Reference

**a2a** — Manage a2a


**healthz** — Manage healthz

- `ori-pp-cli healthz` — Returns 200 with `{ok: true}` when the launchd-managed a2a-server is responding. The most common failure mode is the...

**well-known** — Manage well known

- `ori-pp-cli well-known` — Returns the agent name list configured via OPENCLAW_AGENTS_CONFIG (~/.openclaw/agents.toml). Typically returns...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ori-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find every task that hit input_required this week

```bash
ori tasks list --agent ori --state input_required --since 7d --json --select task_id,context_id,updated_at,text
```

Dotted --select narrows the payload to the four fields an agent or jq pipeline actually needs; the full Task includes accumulated text that can be tens of KB.

### One-shot health check from cron

```bash
ori doctor --json | jq -e '.checks | all(.ok == true)'
```

Exit 0 if every diagnostic passed, non-zero otherwise. Pipes straight into a cron job or a status-page poller.

### Resume a wedged task by context

```bash
ori contexts list --agent ori --since 1h --json | jq -r '.[0].last_task_id' | xargs -I {} ori tasks resume {} --agent ori
```

When the main Claude context overflowed and you lost the task_id, find the most recent context and resume its last task by id.

### Decide every pending approval interactively

```bash
ori approvals pending --watch
```

Live polling across both agents; prints each approval as it arrives. Pair with `ori approvals respond <id> --decision approve` in a second terminal.

### Tail stderr during a bridge incident

```bash
ori logs tail --stream stderr --lines 200
```

Hides the ~/Library/Logs/openclaw-a2a-server/stderr.log path; one verb during a wedge.

## Auth Setup

No authentication required.

Run `ori-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ori-pp-cli healthz --agent --select id,name,status
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
ori-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ori-pp-cli feedback --stdin < notes.txt
ori-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ori-pp-cli/feedback.jsonl`. They are never POSTed unless `ORI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ORI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ori-pp-cli profile save briefing --json
ori-pp-cli --profile briefing healthz
ori-pp-cli profile list --json
ori-pp-cli profile show briefing
ori-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ori-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ori-pp-mcp -- ori-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ori-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ori-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ori-pp-cli <command> --help`.
