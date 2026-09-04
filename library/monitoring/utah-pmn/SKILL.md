---
name: pp-utah-pmn
description: "One command to sweep Utah public-meeting notices for land-use approvals — county-wide, agenda-scanned, and diffed against what you've already seen. Trigger phrases: `what land-use meetings are coming up in Millard County`, `check Delta City planning meetings`, `any new public notices in Fillmore`, `scan agendas for rezoning`, `use utah-pmn`, `run utah-pmn`."
author: "Paul Gradeff"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - utah-pmn-pp-cli
    install:
      - kind: go
        bins: [utah-pmn-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/cmd/utah-pmn-pp-cli
---

# Utah Public Notice Website — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `utah-pmn-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install utah-pmn --cli-only
   ```
2. Verify: `utah-pmn-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/cmd/utah-pmn-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

The state Public Notice Website makes you pick a ZIP and click through bodies one at a time. This CLI sweeps a whole county's towns in one call, keeps only planning/council/commission/board meetings, scans the inline agendas for rezones, subdivisions, CUPs, and variances, and with a local store tells you what's new since your last run. Built for tracking Delta City and Millard County development approvals.

## When to Use This CLI

Use this CLI to monitor Utah public-body meetings for land-use and development approvals, especially across Delta City and Millard County. It is ideal for recurring/scheduled checks where you only want new meetings, and for cutting a body's calendar down to zoning/subdivision/CUP items.

## Anti-triggers

Do not use this CLI for:
- Do not use this for meeting notices outside Utah — the source is the Utah-only state registry.
- Do not use this to register or submit public comment — it is read-only monitoring.
- Do not rely on it for legally-binding minutes; always confirm against the official notice detail page.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### County-wide land-use monitoring
- **`millard`** — Sweep every Millard County town in one command and keep only land-use approval bodies (planning commissions, councils, county commission, boards).

  _Pick this when the user asks what development/zoning meetings are coming up in Millard County._

  ```bash
  utah-pmn-pp-cli millard --days 30 --agent
  ```
- **`landuse`** — Filter any notice set to meetings whose body or agenda involves zoning, subdivisions, rezones, CUPs, variances, annexations, or plats.

  _Pick this to cut a body's full calendar down to only the approval items that matter._

  ```bash
  utah-pmn-pp-cli landuse --location Delta --days 60 --agent
  ```
- **`agenda scan`** — Search the inline agenda text of upcoming notices for arbitrary terms (e.g. a project name, a parcel, an applicant), with surrounding context.

  _Pick this to find every meeting touching a specific project, parcel, or applicant._

  ```bash
  utah-pmn-pp-cli agenda scan "solar" --agent
  ```
- **`locations`** — List the curated Millard County town/ZIP set the sweep uses, so the coverage is transparent and editable.

  _Pick this to see or confirm which towns a county sweep covers._

  ```bash
  utah-pmn-pp-cli locations --agent
  ```

### Local state that compounds
- **`since`** — Show only notices first seen since the last run, so a scheduled run reports just what changed. The CLI records what it has seen locally; it does not depend on a separate sync step.

  _Pick this for a recurring/scheduled check so the user only sees new meetings._

  ```bash
  utah-pmn-pp-cli since --location Fillmore --agent
  ```
- **`watch`** — Track a saved list of public bodies (e.g. Delta City Council, Millard County Planning Commission) and report their upcoming meetings across locations.

  _Pick this when the user cares about specific bodies rather than a whole county._

  ```bash
  utah-pmn-pp-cli watch list --agent
  ```

## Command Reference

**notice** — A single notice detail page (full agenda, minutes, attached materials)

- `utah-pmn-pp-cli notice` — Fetch the full HTML detail page for a notice, including minutes and material links

**notices** — Public meeting notices published by Utah public bodies (agendas inline)

- `utah-pmn-pp-cli notices` — List meeting notices near a ZIP code or city, within a date range.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
utah-pmn-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Monthly county scan

```bash
utah-pmn-pp-cli millard --days 30 --agent
```

All land-use bodies meeting in Millard County over the next month, structured for an agent.

### New since last check

```bash
utah-pmn-pp-cli since --location Fillmore --agent
```

Only notices first seen since the previous run — ideal on a schedule.

### Find a project by name

```bash
utah-pmn-pp-cli agenda scan "subdivision" --agent --select noticeId,publicBodyName,meetingStartTime
```

Every stored agenda mentioning the term, narrowed to the key fields.

## Auth Setup

No authentication required.

Run `utah-pmn-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  utah-pmn-pp-cli notice --notice-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
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

- Use `--home <dir>` for one invocation, or set `UTAH_PMN_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `UTAH_PMN_CONFIG_DIR`, `UTAH_PMN_DATA_DIR`, `UTAH_PMN_STATE_DIR`, `UTAH_PMN_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `UTAH_PMN_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `utah-pmn-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "utah-pmn": {
        "command": "utah-pmn-pp-mcp",
        "env": {
          "UTAH_PMN_HOME": "/srv/utah-pmn"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `UTAH_PMN_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `UTAH_PMN_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
utah-pmn-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
utah-pmn-pp-cli feedback --stdin < notes.txt
utah-pmn-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `UTAH_PMN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `UTAH_PMN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
utah-pmn-pp-cli profile save briefing --json
utah-pmn-pp-cli --profile briefing notice --notice-id 550e8400-e29b-41d4-a716-446655440000
utah-pmn-pp-cli profile list --json
utah-pmn-pp-cli profile show briefing
utah-pmn-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `utah-pmn-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/cmd/utah-pmn-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add utah-pmn-pp-mcp -- utah-pmn-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which utah-pmn-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   utah-pmn-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `utah-pmn-pp-cli <command> --help`.
