---
name: pp-alltrails
description: "Printing Press CLI for Alltrails. Evidence-labeled, live-capable route map for AllTrails browser/mobile surfaces. Not an official API contract."
author: "zaydiscold"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - alltrails-pp-cli
---

# Alltrails — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `alltrails-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install alltrails --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `alltrails-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/alltrails/cmd/alltrails-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## HTTP Transport

This CLI uses standard Go HTTP transport with HTTP/2 enabled when AllTrails negotiates it. It does not require a resident browser process for normal API calls.

## Command Reference

**alltrails** — Manage alltrails

- `alltrails-pp-cli alltrails create` — Upload a new activity recording
- `alltrails-pp-cli alltrails get` — Activity detail with GPS/stat surfaces
- `alltrails-pp-cli alltrails get-v3` — Trail detail payload
- `alltrails-pp-cli alltrails get-v3-2` — Offline map metadata
- `alltrails-pp-cli alltrails get-v3-3` — Map static image metadata/render endpoint
- `alltrails-pp-cli alltrails get-v3-4` — Trail static map image metadata/render endpoint
- `alltrails-pp-cli alltrails get-v3-5` — User activity list
- `alltrails-pp-cli alltrails list` — Trail search by text, location, and filters
- `alltrails-pp-cli alltrails list-v3` — Current account profile


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
alltrails-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Public reads may work without credentials. Account routes use caller-owned credentials:

- `ALLTRAILS_ACCESS_TOKEN` — Bearer token for authenticated API requests.
- `ALLTRAILS_COOKIE` — browser Cookie header for authenticated routes.
- `ALLTRAILS_CSRF_TOKEN` — CSRF header for browser-backed write routes when required.

Run `alltrails-pp-cli doctor` to verify setup.

## Write Safety

Default to read commands. Write routes are annotated with `mcp:read-only=false` and `mcp:risk=<level>`.

For PP-side safety, live AllTrails writes are blocked unless both conditions are true:

1. The command is called with `--live-write` instead of the default write dry-run.
2. `ALLTRAILS_PP_ALLOW_WRITES=1` is present in the environment.

If either condition is missing, write routes preview the request or fail closed before HTTP transport. Do not set `ALLTRAILS_PP_ALLOW_WRITES` unless the user has explicitly approved the exact live account mutation.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  alltrails-pp-cli alltrails list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

`sync` defaults to a safe no-op because AllTrails browser APIs are DataDome-protected outside the logged-in browser context. Use `sync --resources routing_info` only when caller-owned auth/cookies are configured and a live read is explicitly desired.

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
alltrails-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
alltrails-pp-cli feedback --stdin < notes.txt
alltrails-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/alltrails-pp-cli/feedback.jsonl`. They are never POSTed unless `ALLTRAILS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ALLTRAILS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
alltrails-pp-cli profile save briefing --json
alltrails-pp-cli --profile briefing alltrails list
alltrails-pp-cli profile list --json
alltrails-pp-cli profile show briefing
alltrails-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `alltrails-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add alltrails-pp-mcp -- alltrails-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which alltrails-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   alltrails-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `alltrails-pp-cli <command> --help`.
