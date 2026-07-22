---
name: pp-peloton
description: "Printing Press CLI for Peloton. Read-only Peloton workout, class, and structural-provider facts in a private local store."
author: "Felix Banuchi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - peloton-pp-cli
    install:
      - kind: go
        bins: [peloton-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/health/peloton/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Peloton — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `peloton-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install peloton --cli-only
   ```
2. Verify: `peloton-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Read-only Peloton workout, class, and structural-provider facts in a private local store.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**account** — Current account/profile fact; no implicit account expansion.

- `peloton-pp-cli account` — Show the current profile fact.

**classes** — Read-only catalog, class detail, planned structure, and provider filter vocabulary.

- `peloton-pp-cli classes catalog` — List a caller-scoped archived class catalog page.
- `peloton-pp-cli classes filters` — Show provider class/filter vocabulary and embedded instructor metadata.
- `peloton-pp-cli classes search` — Search the caller-scoped catalog by factual provider filters; U4 adds offline structural predicates.
- `peloton-pp-cli classes show` — Show class metadata and supported planned structure.
- `peloton-pp-cli classes structure` — Inspect ordered provider segments and target ranges without coaching labels.

**strength** — Provider-supplied performed movement facts present only in workout detail payloads.

- `peloton-pp-cli strength <workout_id>` — Inspect provider workout detail containing movement_tracker_data when present; no template fallback.

**workouts** — Read-only recorded workout history, detail, and recorded performance facts.

- `peloton-pp-cli workouts list` — List workout history in newest-first pages; user_id is supplied by the caller until U3 links the profile fact.
- `peloton-pp-cli workouts performance` — Show recorded performance samples and summaries for one workout.
- `peloton-pp-cli workouts show` — Show a recorded workout detail payload.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
peloton-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

This CLI uses OAuth2 with refresh-token rotation. Configure the client credentials and refresh token:

```bash
export PELOTON_OAUTH_USERNAME="your-username"
export PELOTON_OAUTH_PASSWORD="your-password"
```

Access tokens are refreshed automatically before API calls.

Run `peloton-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  peloton-pp-cli classes search --browse-category example-value --content-format example-value --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `PELOTON_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `PELOTON_CONFIG_DIR`, `PELOTON_DATA_DIR`, `PELOTON_STATE_DIR`, `PELOTON_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `PELOTON_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `peloton-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "peloton": {
        "command": "peloton-pp-mcp",
        "env": {
          "PELOTON_HOME": "/srv/peloton"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `PELOTON_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `PELOTON_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
peloton-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
peloton-pp-cli feedback --stdin < notes.txt
peloton-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `PELOTON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PELOTON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

```
peloton-pp-cli profile save briefing --json
peloton-pp-cli --profile briefing classes search --browse-category example-value --content-format example-value
peloton-pp-cli profile list --json
peloton-pp-cli profile show briefing
peloton-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `peloton-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/health/peloton/cmd/peloton-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add peloton-pp-mcp -- peloton-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which peloton-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   peloton-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `peloton-pp-cli <command> --help`.
