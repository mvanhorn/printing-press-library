---
name: pp-sncf-connect
description: "Printing Press CLI for Sncf Connect. navitia.io is the open API for building cool stuff with mobility data."
author: "jmbernabotto"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sncf-connect-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/sncf-connect/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Sncf Connect — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sncf-connect-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install sncf-connect --cli-only
   ```
2. Verify: `sncf-connect-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

navitia.io is the open API for building cool stuff with mobility data. It provides the following services

    * journeys computation
    * line schedules
    * next departures
    * exploration of public transport data / search places
    * and sexy things such as isochrones

    navitia is a HATEOAS API that returns JSON formated results

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**coord** — Manage coord

- `sncf-connect-pp-cli coord <lon> <lat>` — Get lon lat

**coords** — Manage coords

- `sncf-connect-pp-cli coords <lon> <lat>` — Get lon lat

**coverage** — Manage coverage

- `sncf-connect-pp-cli coverage get` — Get
- `sncf-connect-pp-cli coverage get-lon-lat` — Get lon lat
- `sncf-connect-pp-cli coverage get-region` — Get region

**elevations** — Manage elevations

- `sncf-connect-pp-cli elevations` — Get

**journeys** — Manage journeys

- `sncf-connect-pp-cli journeys` — Get

**line-groups** — Manage line groups

- `sncf-connect-pp-cli line-groups` — Get

**lines** — Manage lines

- `sncf-connect-pp-cli lines` — Get

**networks** — Manage networks

- `sncf-connect-pp-cli networks` — Get

**places** — Manage places

- `sncf-connect-pp-cli places get` — Get
- `sncf-connect-pp-cli places get-id` — Get id

**route-schedules** — Manage route schedules

- `sncf-connect-pp-cli route-schedules` — Get

**routes** — Manage routes

- `sncf-connect-pp-cli routes` — Get

**stop-areas** — Manage stop areas

- `sncf-connect-pp-cli stop-areas` — Get

**stop-points** — Manage stop points

- `sncf-connect-pp-cli stop-points` — Get

**stop-schedules** — Manage stop schedules

- `sncf-connect-pp-cli stop-schedules` — Get

**terminus-schedules** — Manage terminus schedules

- `sncf-connect-pp-cli terminus-schedules` — Get

**vehicle-journeys** — Manage vehicle journeys

- `sncf-connect-pp-cli vehicle-journeys` — Get


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sncf-connect-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `sncf-connect-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export SNCF_API_KEY="<your-key>"
```

Or persist it in `~/.config/navitia-pp-cli/config.toml`.

Run `sncf-connect-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sncf-connect-pp-cli coverage get --agent --select id,name,status
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
sncf-connect-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sncf-connect-pp-cli feedback --stdin < notes.txt
sncf-connect-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.sncf-connect-pp-cli/feedback.jsonl`. They are never POSTed unless `SNCF_CONNECT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SNCF_CONNECT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
sncf-connect-pp-cli profile save briefing --json
sncf-connect-pp-cli --profile briefing coverage get
sncf-connect-pp-cli profile list --json
sncf-connect-pp-cli profile show briefing
sncf-connect-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `sncf-connect-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add sncf-connect-pp-mcp -- sncf-connect-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sncf-connect-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sncf-connect-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sncf-connect-pp-cli <command> --help`.
